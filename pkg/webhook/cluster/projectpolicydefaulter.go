// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"errors"
	"fmt"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
	nutanixclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/helpers/nutanix"
	preflightnutanix "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/nutanix"
)

// pcVersionResolver returns the raw Prism Central version for a cluster.
type pcVersionResolver func(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	spec *carenv1.NutanixClusterConfigSpec,
) (string, error)

type projectPolicyDefaulter struct {
	client                  ctrlclient.Client
	pcVersionFn             pcVersionResolver
	resolveProjectFn        projectResolver
	resolveDefaultProjectFn defaultProjectResolver
}

func NewProjectPolicyDefaulter(client ctrlclient.Client) *projectPolicyDefaulter {
	defaulter := &projectPolicyDefaulter{
		client:      client,
		pcVersionFn: nutanixclient.PrismCentralVersionForCluster,
	}
	defaulter.resolveProjectFn = defaulter.resolveProjectForPolicy
	defaulter.resolveDefaultProjectFn = defaulter.resolvePCDefaultProjectForPolicy
	return defaulter
}

type projectResolver func(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	spec *carenv1.NutanixClusterConfigSpec,
	id *capxv1.NutanixResourceIdentifier,
	pcVersion string,
) (resolvedUUID, resolvedName string, err error)

type defaultProjectResolver func(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	spec *carenv1.NutanixClusterConfigSpec,
	pcVersion string,
) (defaultUUID, defaultName string, err error)

const (
	projectUUIDAnnotationKey = "capx.nutanix.com/project-uuid"
	policySingleProject      = "single-project"
)

// mutateCluster sets the Nutanix project policy annotation (in-memory) on the cluster.
//
// The annotation records a one-time, irreversible tenancy decision: a cluster's project is a hard
// tenant boundary that can never change after creation. Because of this, the annotation is the source
// of truth and is frozen once set; it is never recomputed from live Prism Central / CSI versions.
//
// CSI (presence or version) does not factor into the decision: the Nutanix CSI version is pinned to the
// CAREN/NKP release, so the policy is a property of the release, driven only by the Prism Central version
// and whether a project is in use.
//
// Behavior:
//   - Paused clusters are skipped entirely: pausing, moving, or restoring a cluster must never trigger a
//     Prism Central call, so these operations keep working while Prism Central is unavailable. A genuinely
//     new cluster is created unpaused, so the create-time enforcement still applies to it; a paused create
//     is a move/restore that carries its (frozen) annotation, or is back-filled on its first unpaused update.
//   - If the policy annotation is already set to default-only or single-project, it is left untouched
//     (frozen). unrestricted is re-evaluated on UPDATE so a cluster that was admitted without a
//     tenancy boundary can be narrowed once its placement is known.
//   - On CREATE, the control-plane project alone selects the policy, on every Prism Central version:
//   - no control-plane project, or one resolving to the default project => default-only
//   - any other project => single-project + project-uuid
//   - On UPDATE back-fill (annotation absent) and unrestricted re-evaluation:
//   - no project references => default-only
//   - one distinct project that is the default => default-only
//   - one distinct non-default project named by every Nutanix machine spec => single-project
//   - one distinct non-default project named by only some specs, or multiple projects => unrestricted
//   - a missing credentials Secret fails closed rather than leaving or writing unrestricted
//   - a default project that exists but is not accessible (RBAC) is treated as not-default, not as a failure
//
// CREATE always looks up Prism Central version and fails closed if it cannot. UPDATE looks it up only when
// project resolution will run (backfill or unrestricted re-eval with project refs). Frozen policies, paused
// clusters, and no-ref default-only backfill do not. Version does not select the policy tree; it is passed
// into project resolution (v4 on 7.6+, v3 below).
//
// It fails closed: if the required inputs cannot be determined, it returns an error so the admission
// request is rejected rather than persisting a wrong, irreversible policy.
func (a *projectPolicyDefaulter) mutateCluster(
	ctx context.Context,
	req admission.Request,
	cluster *clusterv1.Cluster,
) error {
	log := ctrl.LoggerFrom(ctx).WithName("cluster/project-policy")

	if req.Operation == admissionv1.Delete {
		return nil
	}
	if cluster.GetDeletionTimestamp() != nil {
		return nil
	}
	if !cluster.Spec.Topology.IsDefined() {
		return nil
	}

	// Never do policy work (and never call Prism Central) for a paused cluster: pause / clusterctl move /
	// restore must succeed even when Prism Central is unavailable. New clusters are created unpaused, so the
	// create-time PC >= 7.6 enforcement is unaffected; annotation-less paused clusters are back-filled on
	// their first unpaused update.
	if ptr.Deref(cluster.Spec.Paused, false) {
		return nil
	}

	// CSI storage-project-policy is independent of CAPX project-policy. Set it before the frozen
	// CAPX-policy return so an existing Cluster still gets default on UPDATE when the key is missing.
	a.setStoragePolicy(req, cluster)

	key := nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey
	// The annotation is frozen once set, with one exception: unrestricted is the least specific policy
	// and imposes no tenancy boundary, so an update re-evaluates it in case the cluster has since become
	// describable by a narrower one. Every other value is a committed boundary and is never recomputed.
	if existing, ok := cluster.Annotations[key]; ok {
		if req.Operation != admissionv1.Update || existing != nutanixprojectpolicy.PolicyUnrestricted {
			return nil
		}
	}

	clusterConfigVar := variables.GetClusterVariableByName(
		carenv1.ClusterConfigVariableName,
		cluster.Spec.Topology.Variables,
	)
	if clusterConfigVar == nil {
		return nil
	}

	nutanixClusterConfigSpec := &carenv1.NutanixClusterConfigSpec{}
	if err := variables.UnmarshalClusterVariable(clusterConfigVar, nutanixClusterConfigSpec); err != nil {
		return fmt.Errorf("failed to unmarshal clusterConfig for project policy annotation: %w", err)
	}

	// Only Nutanix clusters carry a project policy.
	if nutanixClusterConfigSpec.Nutanix == nil &&
		(nutanixClusterConfigSpec.ControlPlane == nil || nutanixClusterConfigSpec.ControlPlane.Nutanix == nil) {
		return nil
	}

	desired, desiredProjectUUID, err := a.computeDesiredPolicy(
		ctx,
		req.Operation,
		cluster,
		nutanixClusterConfigSpec,
	)
	if err != nil {
		return err
	}

	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string, 1)
	}
	cluster.Annotations[key] = desired
	if desired == policySingleProject {
		cluster.Annotations[projectUUIDAnnotationKey] = desiredProjectUUID
	}

	log.V(1).Info("set Nutanix project policy annotation",
		"value", desired,
		"projectUUID", desiredProjectUUID,
		"operation", req.Operation,
	)
	return nil
}

// setStoragePolicy writes csi.nutanix.com/storage-project-policy only when the key is missing.
// CREATE sets project-policy (new clusters take the CSI / project-scoped AOS checks). UPDATE
// sets default (brownfield). An already-set value is left alone. Paused clusters never reach here.
func (a *projectPolicyDefaulter) setStoragePolicy(req admission.Request, cluster *clusterv1.Cluster) {
	key := nutanixprojectpolicy.CSIStorageProjectPolicyAnnotationKey
	if cluster.Annotations != nil && strings.TrimSpace(cluster.Annotations[key]) != "" {
		return
	}
	if cluster.Annotations == nil {
		cluster.Annotations = make(map[string]string, 1)
	}
	switch req.Operation {
	case admissionv1.Create:
		cluster.Annotations[key] = nutanixprojectpolicy.StoragePolicyProjectPolicy
	case admissionv1.Update:
		cluster.Annotations[key] = nutanixprojectpolicy.StoragePolicyDefault
	}
}

// panicToPolicyError converts a recovered panic from the Prism Central resolution path into a readable,
// fail-closed error.
//
// The generated Nutanix SDK panics with an "interface conversion: *http.Response, not []uint8" type
// assertion whenever Prism Central returns a non-JSON response (Content-Type text/plain or text/html):
// the SDK returns the raw *http.Response before its status-code check, then blindly asserts it to []byte.
// In practice that only happens on error/redirect responses - most commonly a 401/403 auth failure or an
// auth proxy in front of Prism Central - so we surface an actionable, credentials-oriented message for that
// signature while still including the raw panic detail for reporting.
func panicToPolicyError(r any) error {
	detail := fmt.Sprintf("%v", r)
	if strings.Contains(detail, "http.Response") {
		return fmt.Errorf(
			"received an unexpected non-JSON response from Prism Central; verify the endpoint, credentials, and permissions (detail: %s)", //nolint:lll // Message is long.
			detail,
		)
	}
	return fmt.Errorf("internal error (panic) while defaulting the project policy: %s", detail)
}

// computeDesiredPolicy returns the project policy annotation value for the cluster, or an error if it
// cannot be determined (so the caller can fail closed).
func (a *projectPolicyDefaulter) computeDesiredPolicy(
	ctx context.Context,
	operation admissionv1.Operation,
	cluster *clusterv1.Cluster,
	nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec,
) (policy, projectUUID string, err error) {
	// Prism Central resolution calls into the generated Nutanix SDK, which panics (e.g. an
	// "interface conversion: *http.Response, not []uint8" type assertion) when Prism Central returns a
	// non-JSON response such as an HTML auth/error page. Recover here so the panic becomes a normal,
	// fail-closed admission error with a readable message instead of crashing the webhook. This mirrors the
	// per-check recovery already in the preflight validating webhook.
	defer func() {
		if r := recover(); r != nil {
			err = panicToPolicyError(r)
		}
	}()

	// The Nutanix CSI version is pinned to the CAREN/NKP release, so the policy is a property of the
	// release rather than of an individual cluster. CSI (presence or version) deliberately does not factor
	// into the decision; it is driven solely by the Prism Central version and whether a project is in use.
	if operation == admissionv1.Create {
		pcRaw, err := a.pcVersionFn(ctx, a.client, cluster, nutanixClusterConfigSpec)
		if err != nil {
			return "", "", fmt.Errorf("failed to resolve Prism Central version for project policy annotation: %w", err)
		}
		if pcRaw == "" {
			return "", "", fmt.Errorf("could not determine Prism Central version for project policy annotation")
		}

		// The policy is selected from the control-plane project alone, on every Prism Central version.
		// Version differences are confined to project resolution itself (v4 on 7.6+, v3 below), so the
		// decision here is the same either way.
		return a.policyFromControlPlaneProject(ctx, cluster, nutanixClusterConfigSpec, pcRaw)
	}

	return a.policyForBrownfieldBackfill(ctx, cluster, nutanixClusterConfigSpec)
}

// policyFromControlPlaneProject selects the policy from the control-plane project reference.
//
// No control-plane project, or one that resolves to the Prism Central default project, means the
// cluster stays on the default project => default-only. Any other project => single-project, pinned
// to that project's UUID.
//
// Project and default-project resolution is version-aware internally: Prism Central 7.6+ uses the
// v4 Projects API. On earlier versions a resolved project named _internal is the default; no extra
// default-project API call is made. Both return the same shape, so the comparison below is
// identical on every version.
func (a *projectPolicyDefaulter) policyFromControlPlaneProject(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec,
	pcVersion string,
) (policy, projectUUID string, err error) {
	cpProject := controlPlaneProjectRef(nutanixClusterConfigSpec)
	if cpProject == nil {
		return nutanixprojectpolicy.PolicyDefaultOnly, "", nil
	}

	projectUUID, projectName, err := a.resolveProjectFn(
		ctx,
		cluster,
		nutanixClusterConfigSpec,
		cpProject,
		pcVersion,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve project for project policy annotation: %w", err)
	}
	projectUUID = strings.TrimSpace(projectUUID)
	if projectUUID == "" {
		return "", "", fmt.Errorf("resolved project UUID is empty for project policy annotation")
	}

	if !nutanixprojectpolicy.PCVersionAtLeast76(pcVersion) {
		if preflightnutanix.IsV3DefaultProjectName(projectName) {
			return nutanixprojectpolicy.PolicyDefaultOnly, "", nil
		}
		return policySingleProject, projectUUID, nil
	}

	defaultUUID, defaultName, err := a.resolveDefaultProjectFn(ctx, cluster, nutanixClusterConfigSpec, pcVersion)
	if err != nil || strings.TrimSpace(defaultUUID) == "" {
		// A missing or undiscoverable default project must not be treated as a match.
		return policySingleProject, projectUUID, nil
	}

	if projectMatchesDefault(projectUUID, projectName, defaultUUID, defaultName) {
		return nutanixprojectpolicy.PolicyDefaultOnly, "", nil
	}
	return policySingleProject, projectUUID, nil
}

// policyForBrownfieldBackfill selects the policy for a cluster that has no policy yet, or whose policy
// is unrestricted and is being re-evaluated on update.
//
// A machine spec with no project reference runs in the Prism Central default project, so absence is a
// project choice rather than missing information. That is what distinguishes the cases below:
//
//   - no project references at all => every machine is in the default project => default-only
//   - more than one distinct project => the cluster spans projects => unrestricted
//   - one distinct project, and it is the default => default-only, whether or not every spec names it,
//     since the specs that do not name a project run in the default project anyway
//   - one distinct non-default project named by every Nutanix machine spec => single-project, pinned to it
//   - one distinct non-default project named by only some specs => the rest run in the default project,
//     so the cluster spans both => unrestricted
func (a *projectPolicyDefaulter) policyForBrownfieldBackfill(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	nutanixClusterConfigSpec *carenv1.NutanixClusterConfigSpec,
) (policy, projectUUID string, err error) {
	workerSpecs, err := collectWorkerNodeConfigSpecs(cluster)
	if err != nil {
		return "", "", err
	}
	refs := nutanixprojectpolicy.CollectReferencedProjects(nutanixClusterConfigSpec, workerSpecs)
	if len(refs) == 0 {
		return nutanixprojectpolicy.PolicyDefaultOnly, "", nil
	}

	pcRaw, err := a.pcVersionFn(ctx, a.client, cluster, nutanixClusterConfigSpec)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve Prism Central version for project policy annotation: %w", err)
	}
	if pcRaw == "" {
		return "", "", fmt.Errorf("could not determine Prism Central version for project policy annotation")
	}

	uniqueProjects := map[string]string{}
	for _, ref := range refs {
		resolvedUUID, resolvedName, resolveErr := a.resolveProjectFn(
			ctx,
			cluster,
			nutanixClusterConfigSpec,
			ref,
			pcRaw,
		)
		if resolveErr != nil {
			return "", "", fmt.Errorf("failed to resolve project for project policy annotation: %w", resolveErr)
		}
		resolvedUUID = strings.TrimSpace(resolvedUUID)
		if resolvedUUID == "" {
			return "", "", fmt.Errorf("resolved project UUID is empty for project policy annotation")
		}
		uniqueProjects[resolvedUUID] = strings.TrimSpace(resolvedName)
	}

	if len(uniqueProjects) > 1 {
		return nutanixprojectpolicy.PolicyUnrestricted, "", nil
	}

	var onlyUUID, onlyName string
	for u, n := range uniqueProjects {
		onlyUUID, onlyName = u, n
	}

	// A machine spec that names no project runs in the default project, so a cluster whose single named
	// project is the default one sits entirely in the default project whatever its coverage. Checking
	// this before coverage keeps full and partial coverage from disagreeing about identical placement.
	// If the default project is not accessible, skip the default-only match and decide from coverage.
	defaultUUID, defaultName, err := a.resolveDefaultProjectFn(ctx, cluster, nutanixClusterConfigSpec, pcRaw)
	if err != nil && !errors.Is(err, preflightnutanix.ErrProjectNotAccessible) {
		return "", "", fmt.Errorf(
			"failed to resolve the Prism Central default project for project policy annotation: %w",
			err,
		)
	}
	if err == nil && projectMatchesDefault(onlyUUID, onlyName, defaultUUID, defaultName) {
		return nutanixprojectpolicy.PolicyDefaultOnly, "", nil
	}

	// A non-default project pins the cluster only when every Nutanix machine spec names it. Otherwise
	// the unreferenced specs run in the default project and the cluster spans both.
	if len(refs) == countNutanixMachineSpecs(nutanixClusterConfigSpec, workerSpecs) {
		return policySingleProject, onlyUUID, nil
	}
	return nutanixprojectpolicy.PolicyUnrestricted, "", nil
}

// countNutanixMachineSpecs counts the machine specs that carry Nutanix configuration: the control plane
// plus every Nutanix worker pool. It mirrors what CollectReferencedProjects iterates, so comparing the
// two reveals how many of those specs left the project unset.
func countNutanixMachineSpecs(
	spec *carenv1.NutanixClusterConfigSpec,
	workerSpecs map[string]*carenv1.NutanixWorkerNodeConfigSpec,
) int {
	count := 0
	if spec != nil && spec.ControlPlane != nil && spec.ControlPlane.Nutanix != nil {
		count++
	}
	for _, ws := range workerSpecs {
		if ws == nil || ws.Nutanix == nil {
			continue
		}
		count++
	}
	return count
}

func controlPlaneProjectRef(spec *carenv1.NutanixClusterConfigSpec) *capxv1.NutanixResourceIdentifier {
	if spec == nil || spec.ControlPlane == nil || spec.ControlPlane.Nutanix == nil {
		return nil
	}
	return spec.ControlPlane.Nutanix.MachineDetails.Project
}

func projectMatchesDefault(
	projectUUID, projectName, defaultUUID, defaultName string,
) bool {
	projectUUID = strings.TrimSpace(projectUUID)
	projectName = strings.TrimSpace(projectName)
	defaultUUID = strings.TrimSpace(defaultUUID)
	defaultName = strings.TrimSpace(defaultName)

	return (projectUUID != "" && defaultUUID != "" && strings.EqualFold(projectUUID, defaultUUID)) ||
		(projectName != "" && defaultName != "" && strings.EqualFold(projectName, defaultName))
}

func (a *projectPolicyDefaulter) resolveProjectForPolicy(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	spec *carenv1.NutanixClusterConfigSpec,
	id *capxv1.NutanixResourceIdentifier,
	pcVersion string,
) (uuid, name string, err error) {
	if cluster == nil || spec == nil || spec.Nutanix == nil {
		return "", "", fmt.Errorf("cluster or Nutanix cluster config is not initialized")
	}
	creds, trustPEM, err := preflightnutanix.LoadPrismCredentialBundle(ctx, a.client, cluster.Namespace, spec)
	if err != nil {
		return "", "", err
	}
	nclient, err := preflightnutanix.NewNutanixPrismClient(
		creds,
		types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace},
		trustPEM,
	)
	if err != nil {
		return "", "", err
	}
	resolved, err := nclient.ResolveProject(ctx, id, pcVersion)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(resolved.UUID), strings.TrimSpace(resolved.Name), nil
}

func (a *projectPolicyDefaulter) resolvePCDefaultProjectForPolicy(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	spec *carenv1.NutanixClusterConfigSpec,
	pcVersion string,
) (uuid, name string, err error) {
	if cluster == nil || spec == nil || spec.Nutanix == nil {
		return "", "", fmt.Errorf("cluster or Nutanix cluster config is not initialized")
	}
	creds, trustPEM, err := preflightnutanix.LoadPrismCredentialBundle(ctx, a.client, cluster.Namespace, spec)
	if err != nil {
		return "", "", err
	}
	nclient, err := preflightnutanix.NewNutanixPrismClient(
		creds,
		types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace},
		trustPEM,
	)
	if err != nil {
		return "", "", err
	}
	def, err := nclient.ResolvePCDefaultProject(ctx, pcVersion)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(def.UUID), strings.TrimSpace(def.Name), nil
}

// projectInUse reports whether any control plane or worker machine details reference a Nutanix project.
// collectWorkerNodeConfigSpecs resolves the effective Nutanix workerConfig for every machine deployment,
// preferring a machine-deployment override and otherwise falling back to the top-level workerConfig.
func collectWorkerNodeConfigSpecs(
	cluster *clusterv1.Cluster,
) (map[string]*carenv1.NutanixWorkerNodeConfigSpec, error) {
	out := make(map[string]*carenv1.NutanixWorkerNodeConfigSpec)
	if !cluster.Spec.Topology.IsDefined() {
		return out, nil
	}

	for i := range cluster.Spec.Topology.Workers.MachineDeployments {
		md := &cluster.Spec.Topology.Workers.MachineDeployments[i]

		var workerConfigVar *clusterv1.ClusterVariable
		if len(md.Variables.Overrides) > 0 {
			workerConfigVar = variables.GetClusterVariableByName(
				carenv1.WorkerConfigVariableName,
				md.Variables.Overrides,
			)
		}
		if workerConfigVar == nil {
			workerConfigVar = variables.GetClusterVariableByName(
				carenv1.WorkerConfigVariableName,
				cluster.Spec.Topology.Variables,
			)
		}
		if workerConfigVar == nil {
			continue
		}

		nutanixWorkerNodeConfigSpec := &carenv1.NutanixWorkerNodeConfigSpec{}
		if err := variables.UnmarshalClusterVariable(workerConfigVar, nutanixWorkerNodeConfigSpec); err != nil {
			return nil, fmt.Errorf(
				"failed to unmarshal workerConfig for machineDeployment %q: %w",
				md.Name,
				err,
			)
		}
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			out[md.Name] = nutanixWorkerNodeConfigSpec
		}
	}

	return out, nil
}
