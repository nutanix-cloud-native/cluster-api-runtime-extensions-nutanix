// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	commonv1 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/common/v1/config"
	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/prism-go-client/versionutils"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	// minSupportedAOSVersion is the minimum AOS (Prism Element) version supported by CAREN.
	// Features that depend on newer AOS capabilities add their own, higher floors.
	minSupportedAOSVersion = "7.5"

	// minProjectScopedAOSVersion is the minimum AOS version required for a project-scoped
	// (Projects 2.0) cluster. Project-based data-path operations run on the Prism Element and
	// require AOS >= 7.6, even when Prism Central already advertises 7.6. Admitting a project-scoped
	// cluster whose PE is on AOS 7.5 leads to a confusing late runtime failure (VOL-40601) on the
	// first project-scoped operation. See NCN-116021.
	minProjectScopedAOSVersion = "7.6"

	// minMetroAOSVersion is the minimum AOS version required for a metro (stretch) cluster.
	// Synchronous replication between the two Prism Elements requires AOS >= 7.5.1.
	minMetroAOSVersion = "7.5.1"

	// aosVersionCheckName is the name of the generic minimum-AOS preflight check.
	aosVersionCheckName = "NutanixAOSVersion"
	// projectScopedAOSVersionCheckName is the name of the project-scoped minimum-AOS preflight check.
	projectScopedAOSVersionCheckName = "NutanixProjectScopedAOSVersion"
	// metroAOSVersionCheckName is the name of the metro-cluster minimum-AOS preflight check.
	metroAOSVersionCheckName = "NutanixMetroAOSVersion"

	// Placement-target capability keys used to read a Prism Element's AOS version and name
	// from a project's resource groups. Project-scoped users cannot call the cluster-management
	// API, so the AOS version is fished out of the resource-group placement target instead.
	placementCapabilityKeyVersion     = "version"
	placementCapabilityKeyFullVersion = "full_version"
	placementCapabilityKeyClusterName = "cluster_name"
)

// aosVersionCheck validates that the target Prism Element (AOS) version is at least minVersion.
//
// The target Prism Element is identified either directly by the machine details' cluster
// reference, or indirectly through a failure domain's prismElementCluster. This mirrors the
// resolution used by the storage container and failure domain checks.
type aosVersionCheck struct {
	// machineSpec provides the cluster identifier when no failure domain is configured.
	machineSpec *carenv1.NutanixMachineDetails
	// failureDomainName provides the cluster identifier when a failure domain is configured.
	failureDomainName string
	namespace         string
	kclient           ctrlclient.Client
	nclient           client

	field      string
	name       string
	minVersion string
	pcVersion  string
	projectRef *capxv1.NutanixResourceIdentifier

	// projectScopedOnly limits the check to project-scoped clusters. When true, the check is a no-op
	// for non-project-scoped clusters and only enforces minVersion when the cluster resolves to a
	// project-scoped policy (PC >= 7.6 and a non-default project).
	projectScopedOnly bool

	// metroOnly selects the metro-cluster failure message. The factory already skips non-metro
	// clusters; this flag only changes the user-facing explanation.
	metroOnly bool

	// errMessage is set when the check could not be constructed (e.g. failure domain expansion failed).
	errMessage *string
}

func (c *aosVersionCheck) Name() string {
	if c.name != "" {
		return c.name
	}
	return aosVersionCheckName
}

func (c *aosVersionCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	if c.errMessage != nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Failed to check %s: %s", c.Name(), *c.errMessage),
			Field:   c.field,
		})
		return result
	}

	clusterIdentifier, ok := c.resolveClusterIdentifier(ctx, &result)
	if !ok {
		return result
	}
	// No Prism Element is referenced (for example, placement is entirely by failure domain and the
	// machine details omit a cluster). There is nothing for this check to validate.
	if clusterIdentifier == nil {
		return result
	}

	scopeCtx, err := resolveProjectScopeContext(ctx, c.nclient, c.pcVersion, c.projectRef)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to determine the project scope for Prism Element cluster %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				clusterIdentifier,
				err,
			),
			Field: c.field,
		})
		return result
	}

	// The project-scoped floor only applies when the cluster resolves to a project-scoped policy.
	// For non-project-scoped clusters there is nothing for this check to enforce.
	if c.projectScopedOnly && !isProjectScoped(scopeCtx) {
		return result
	}

	version, err := aosVersionForScope(ctx, c.nclient, scopeCtx, clusterIdentifier)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to determine the AOS version of Prism Element cluster %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				clusterIdentifier,
				err,
			),
			Field: c.field,
		})
		return result
	}

	if !versionutils.Parse(version).AtLeast(versionutils.Parse(c.minVersion)) {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: c.belowMinVersionMessage(clusterIdentifier, version),
			Field:   c.field,
		})
	}

	return result
}

// belowMinVersionMessage builds the failure message for a Prism Element whose AOS version is below
// the required floor, tailoring the guidance to the project-scoped case (NCN-116021).
func (c *aosVersionCheck) belowMinVersionMessage(
	clusterIdentifier *capxv1.NutanixResourceIdentifier,
	version string,
) string {
	if c.projectScopedOnly {
		return fmt.Sprintf(
			"Prism Element cluster %q is running AOS version %q, but a project-scoped (Projects 2.0) cluster requires AOS %s or later. Prism Central advertises project support, but project-based operations run on the Prism Element and will fail at runtime (for example, VOL-40601) until its AOS is upgraded. Upgrade the Prism Element to AOS %s or later, wait for the upgrade to finish, then retry.", //nolint:lll // Message is long.
			clusterIdentifier,
			version,
			c.minVersion,
			c.minVersion,
		)
	}
	if c.metroOnly {
		return fmt.Sprintf(
			"Prism Element cluster %q is running AOS version %q, which is older than the minimum supported version %s for a metro cluster. Upgrade AOS to %s or later, wait for the upgrade to finish, then retry.", //nolint:lll // Message is long.
			clusterIdentifier,
			version,
			c.minVersion,
			c.minVersion,
		)
	}
	return fmt.Sprintf(
		"Prism Element cluster %q is running AOS version %q, which is older than the minimum supported version %s. Upgrade AOS to %s or later, wait for the upgrade to finish, then retry.", //nolint:lll // Message is long.
		clusterIdentifier,
		version,
		c.minVersion,
		c.minVersion,
	)
}

// resolveClusterIdentifier returns the target Prism Element identifier for the check. When a failure
// domain is configured, it fetches the NutanixFailureDomain to read its prismElementCluster. The
// returned bool is false when a failure was recorded on result and the caller should return early.
func (c *aosVersionCheck) resolveClusterIdentifier(
	ctx context.Context,
	result *preflight.CheckResult,
) (*capxv1.NutanixResourceIdentifier, bool) {
	if c.failureDomainName == "" {
		if c.machineSpec == nil {
			return nil, true
		}
		return c.machineSpec.Cluster, true
	}

	fdObj := &capxv1.NutanixFailureDomain{}
	fdKey := ctrlclient.ObjectKey{Name: c.failureDomainName, Namespace: c.namespace}
	if err := c.kclient.Get(ctx, fdKey, fdObj); err != nil {
		result.Allowed = false
		if errors.IsNotFound(err) {
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"NutanixFailureDomain %q was not found in the management cluster. Please create it and retry.", //nolint:lll // Message is long.
					c.failureDomainName,
				),
				Field: c.field,
			})
		} else {
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to get NutanixFailureDomain %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
					c.failureDomainName,
					err,
				),
				Field: c.field,
			})
		}
		return nil, false
	}
	return &fdObj.Spec.PrismElementCluster, true
}

// resolvePrismElementAOSVersion returns the AOS (NOS) version of the target Prism Element.
//
// Non-project-scoped users read the version from the cluster-management API. Project-scoped users
// cannot call that API, so the version is read from the "version" (or "full_version") capability of
// the matching placement target in the project's resource groups.
func resolvePrismElementAOSVersion(
	ctx context.Context,
	nclient client,
	pcVersion string,
	clusterIdentifier *capxv1.NutanixResourceIdentifier,
	projectRef *capxv1.NutanixResourceIdentifier,
) (string, error) {
	if nclient == nil {
		return "", fmt.Errorf("nutanix client is not initialized")
	}
	if clusterIdentifier == nil {
		return "", fmt.Errorf("prism element cluster identifier is empty")
	}

	scopeCtx, err := resolveProjectScopeContext(ctx, nclient, pcVersion, projectRef)
	if err != nil {
		return "", err
	}
	return aosVersionForScope(ctx, nclient, scopeCtx, clusterIdentifier)
}

// isProjectScoped reports whether the resolved scope context is project-scoped.
func isProjectScoped(scopeCtx *projectScopeContext) bool {
	return scopeCtx != nil && scopeCtx.scope == userScopeProjectScoped
}

// aosVersionForScope returns the AOS (NOS) version of the target Prism Element, choosing the read
// path based on the already-resolved project scope. Project-scoped users read the version from the
// project's resource groups; everyone else reads it from the cluster-management API.
func aosVersionForScope(
	ctx context.Context,
	nclient client,
	scopeCtx *projectScopeContext,
	clusterIdentifier *capxv1.NutanixResourceIdentifier,
) (string, error) {
	if isProjectScoped(scopeCtx) {
		return projectScopedPrismElementAOSVersion(ctx, nclient, scopeCtx.projectUUID, clusterIdentifier)
	}

	clusters, err := getClusters(ctx, nclient, clusterIdentifier)
	if err != nil {
		return "", err
	}
	if len(clusters) != 1 {
		return "", fmt.Errorf(
			"found %d Prism Element cluster(s) matching identifier %q, expected exactly 1",
			len(clusters),
			clusterIdentifier,
		)
	}
	version := aosVersionFromCluster(&clusters[0])
	if version == "" {
		return "", fmt.Errorf("no AOS version reported by Prism Element cluster %q", clusterIdentifier)
	}
	return version, nil
}

// projectScopedPrismElementAOSVersion reads the AOS version of a Prism Element from the placement
// targets of a project's resource groups. This is the only path available to project-scoped users,
// who are not authorized to call the cluster-management API.
func projectScopedPrismElementAOSVersion(
	ctx context.Context,
	nclient client,
	projectUUID string,
	clusterIdentifier *capxv1.NutanixResourceIdentifier,
) (string, error) {
	resourceGroups, err := listResourceGroupsForProject(ctx, nclient, projectUUID)
	if err != nil {
		return "", err
	}

	for i := range resourceGroups {
		for j := range resourceGroups[i].PlacementTargets {
			target := &resourceGroups[i].PlacementTargets[j]
			if !placementTargetMatchesCluster(target, clusterIdentifier) {
				continue
			}
			if v := placementTargetAOSVersion(target); v != "" {
				return v, nil
			}
		}
	}

	return "", fmt.Errorf(
		"could not determine the AOS version for Prism Element %q from the project's resource groups",
		clusterIdentifier,
	)
}

// placementTargetAOSVersion returns the AOS version reported by a placement target. The short
// "version" capability is often truncated (7.5.1 reports as "7.5"), so full_version is combined
// via versionutils.ParseReported.
func placementTargetAOSVersion(target *multidomainv4.TargetDetails) string {
	if target == nil {
		return ""
	}
	return versionutils.ParseReported(
		capabilityValue(target.Capabilities, placementCapabilityKeyVersion),
		capabilityValue(target.Capabilities, placementCapabilityKeyFullVersion),
	).String()
}

// placementTargetMatchesCluster reports whether the placement target describes the Prism Element
// referenced by clusterIdentifier, matching by UUID (clusterExtId) or by name (cluster_name).
func placementTargetMatchesCluster(
	target *multidomainv4.TargetDetails,
	clusterIdentifier *capxv1.NutanixResourceIdentifier,
) bool {
	if target == nil || clusterIdentifier == nil {
		return false
	}
	if clusterIdentifier.IsUUID() && clusterIdentifier.UUID != nil {
		return target.ClusterExtId != nil &&
			strings.EqualFold(strings.TrimSpace(*target.ClusterExtId), strings.TrimSpace(*clusterIdentifier.UUID))
	}
	if clusterIdentifier.IsName() && clusterIdentifier.Name != nil {
		name := capabilityValue(target.Capabilities, placementCapabilityKeyClusterName)
		return name != "" && strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(*clusterIdentifier.Name))
	}
	return false
}

// capabilityValue returns the string value of the named capability, or an empty string if it is
// absent or not a string.
func capabilityValue(capabilities []commonv1.KVPair, key string) string {
	for i := range capabilities {
		c := capabilities[i]
		if c.Name != nil && strings.EqualFold(strings.TrimSpace(*c.Name), key) {
			if v, ok := c.GetValue().(string); ok {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

// aosVersionFromCluster returns the AOS (NOS) version reported by a cluster-management Cluster,
// or an empty string when it is not available. Prism often truncates config.buildInfo.version
// (AOS 7.5.1 reports as "7.5"); fullVersion is combined via versionutils.ParseReported.
func aosVersionFromCluster(c *clustermgmtv4.Cluster) string {
	if c == nil || c.Config == nil || c.Config.BuildInfo == nil {
		return ""
	}
	var short, full string
	if c.Config.BuildInfo.Version != nil {
		short = strings.TrimSpace(*c.Config.BuildInfo.Version)
	}
	if c.Config.BuildInfo.FullVersion != nil {
		full = strings.TrimSpace(*c.Config.BuildInfo.FullVersion)
	}
	return versionutils.ParseReported(short, full).String()
}

// identifierKey returns a stable de-duplication key for a Nutanix resource identifier.
func identifierKey(id *capxv1.NutanixResourceIdentifier) string {
	if id == nil {
		return ""
	}
	if id.IsUUID() && id.UUID != nil {
		return "uuid:" + strings.ToLower(strings.TrimSpace(*id.UUID))
	}
	if id.IsName() && id.Name != nil {
		return "name:" + strings.ToLower(strings.TrimSpace(*id.Name))
	}
	return ""
}

func newAOSVersionChecks(cd *checkDependencies) []preflight.Check {
	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return nil
	}
	if cd.nutanixClusterConfigSpec == nil {
		return nil
	}

	b := &aosVersionCheckBuilder{
		cd:         cd,
		name:       aosVersionCheckName,
		minVersion: minSupportedAOSVersion,
		seen:       map[string]struct{}{},
	}
	b.build()
	return b.checks
}

// newProjectScopedAOSVersionChecks adds the project-policy AOS floor (NCN-116021): a project-scoped
// (Projects 2.0) cluster requires the target Prism Element to run AOS >= 7.6. The checks only apply
// when Prism Central is >= 7.6 (the threshold at which the project-policy defaulter marks clusters as
// project-scoped); each check additionally no-ops at runtime for non-project-scoped clusters.
// Clusters annotated csi.nutanix.com/storage-project-policy=default skip this floor (brownfield).
func newProjectScopedAOSVersionChecks(cd *checkDependencies) []preflight.Check {
	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return nil
	}
	if cd.nutanixClusterConfigSpec == nil {
		return nil
	}
	if !nutanixprojectpolicy.PCVersionAtLeast76(cd.pcVersion) {
		return nil
	}
	if cd.cluster != nil && nutanixprojectpolicy.StorageProjectPolicyIsDefault(cd.cluster.Annotations) {
		return nil
	}

	b := &aosVersionCheckBuilder{
		cd:                cd,
		name:              projectScopedAOSVersionCheckName,
		minVersion:        minProjectScopedAOSVersion,
		projectScopedOnly: true,
		seen:              map[string]struct{}{},
	}
	b.build()
	return b.checks
}

// newMetroAOSVersionChecks adds the metro AOS floor (NCN-117247): a metro (stretch) cluster
// requires every target Prism Element to run AOS >= 7.5.1. The checks are only registered when
// the Cluster references a NutanixMetro or NutanixMetroSite failure domain.
func newMetroAOSVersionChecks(cd *checkDependencies) []preflight.Check {
	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return nil
	}
	if cd.nutanixClusterConfigSpec == nil {
		return nil
	}
	if !clusterReferencesMetro(cd) {
		return nil
	}

	b := &aosVersionCheckBuilder{
		cd:         cd,
		name:       metroAOSVersionCheckName,
		minVersion: minMetroAOSVersion,
		metroOnly:  true,
		seen:       map[string]struct{}{},
	}
	b.build()
	return b.checks
}

// clusterReferencesMetro reports whether any control plane or worker failure domain is a
// NutanixMetro or NutanixMetroSite reference.
func clusterReferencesMetro(cd *checkDependencies) bool {
	if cd == nil {
		return false
	}
	found := false
	forEachClusterFailureDomain(cd, func(fd, _ string) {
		if isNutanixMetroFailureDomain(fd) || isNutanixMetroSiteFailureDomain(fd) {
			found = true
		}
	})
	return found
}

// aosVersionCheckBuilder accumulates AOS version checks, de-duplicating by target Prism Element so
// that a Prism Element shared by the control plane and workers is validated only once.
type aosVersionCheckBuilder struct {
	cd                *checkDependencies
	name              string
	minVersion        string
	projectScopedOnly bool
	metroOnly         bool

	seen   map[string]struct{}
	checks []preflight.Check
}

// build walks the control plane and worker machine configurations, adding an AOS version check per
// distinct target Prism Element.
func (b *aosVersionCheckBuilder) build() {
	cd := b.cd

	if cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		cpNutanix := cd.nutanixClusterConfigSpec.ControlPlane.Nutanix
		cpField := "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails.cluster" //nolint:lll // The field is long.
		if len(cpNutanix.FailureDomains) > 0 && cd.cluster != nil && cd.kclient != nil {
			for _, fd := range cpNutanix.FailureDomains {
				b.addFromFailureDomain(fd, cpField, cpNutanix.MachineDetails.Project)
			}
		} else {
			b.addFromMachineDetails(&cpNutanix.MachineDetails, cpField, cpNutanix.MachineDetails.Project)
		}
	}

	for mdName, workerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if workerNodeConfigSpec == nil || workerNodeConfigSpec.Nutanix == nil {
			continue
		}
		machineDetails := &workerNodeConfigSpec.Nutanix.MachineDetails
		if fd, ok := cd.failureDomainByMachineDeploymentName[mdName]; ok && fd != "" &&
			cd.cluster != nil && cd.kclient != nil {
			field := fmt.Sprintf(
				"$.spec.topology.workers.machineDeployments[?@.name==%q].failureDomain",
				mdName,
			)
			b.addFromFailureDomain(fd, field, machineDetails.Project)
		} else {
			field := fmt.Sprintf(
				"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=='%s'].value.nutanix.machineDetails.cluster", //nolint:lll // The field is long.
				mdName,
				carenv1.WorkerConfigVariableName,
			)
			b.addFromMachineDetails(machineDetails, field, machineDetails.Project)
		}
	}
}

func (b *aosVersionCheckBuilder) newCheck() *aosVersionCheck {
	return &aosVersionCheck{
		kclient:           b.cd.kclient,
		nclient:           b.cd.nclient,
		name:              b.name,
		minVersion:        b.minVersion,
		pcVersion:         b.cd.pcVersion,
		projectScopedOnly: b.projectScopedOnly,
		metroOnly:         b.metroOnly,
	}
}

func (b *aosVersionCheckBuilder) addFromMachineDetails(
	machineSpec *carenv1.NutanixMachineDetails,
	field string,
	projectRef *capxv1.NutanixResourceIdentifier,
) {
	if machineSpec == nil || machineSpec.Cluster == nil {
		return
	}
	key := "machine:" + identifierKey(machineSpec.Cluster)
	if key == "machine:" {
		return
	}
	if _, ok := b.seen[key]; ok {
		return
	}
	b.seen[key] = struct{}{}
	check := b.newCheck()
	check.machineSpec = machineSpec
	check.field = field
	check.projectRef = projectRef
	b.checks = append(b.checks, check)
}

func (b *aosVersionCheckBuilder) addFromFailureDomain(
	fd string,
	field string,
	projectRef *capxv1.NutanixResourceIdentifier,
) {
	if fd == "" {
		return
	}
	fdNames, err := getFailureDomainNames(b.cd, fd)
	if err != nil {
		b.cd.log.Error(err, fmt.Sprintf("set the AOS version checkResult for failureDomain %s due to error", fd))
		key := "fd:" + fd
		if _, ok := b.seen[key]; ok {
			return
		}
		b.seen[key] = struct{}{}
		check := b.newCheck()
		check.failureDomainName = fd
		check.namespace = b.cd.cluster.Namespace
		check.field = field
		check.projectRef = projectRef
		check.errMessage = new(err.Error())
		b.checks = append(b.checks, check)
		return
	}

	for _, fdName := range fdNames {
		key := "fd:" + fdName
		if _, ok := b.seen[key]; ok {
			continue
		}
		b.seen[key] = struct{}{}
		check := b.newCheck()
		check.failureDomainName = fdName
		check.namespace = b.cd.cluster.Namespace
		check.field = field
		check.projectRef = projectRef
		b.checks = append(b.checks, check)
	}
}
