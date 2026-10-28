// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
	webhook "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight/skip"
)

const (
	projectConsistencyCheckName      = "NutanixProjectConsistency"
	projectPolicyAnnotationField     = "$.metadata.annotations['capx.nutanix.com/project-policy']"
	singleProjectUUIDAnnotationKey   = "capx.nutanix.com/project-uuid"
	singleProjectUUIDAnnotationField = "$.metadata.annotations['capx.nutanix.com/project-uuid']"
	policySingleProject              = "single-project"
	controlPlaneProjectField         = "$.spec.topology.variables[?@.name==\"clusterConfig\"].value." +
		"controlPlane.nutanix.machineDetails.project"
)

type projectConsistencyCheck struct {
	cd *checkDependencies
}

type policyValidationError struct {
	field   string
	message string
}

func (e *policyValidationError) Error() string {
	return e.message
}

type projectRefWithField struct {
	id    *capxv1.NutanixResourceIdentifier
	field string
}

func newProjectConsistencyCheck(_ context.Context, cd *checkDependencies) preflight.Check {
	return &projectConsistencyCheck{cd: cd}
}

func (c *projectConsistencyCheck) Name() string {
	return projectConsistencyCheckName
}

//nolint:gocyclo // The check walks every project reference in order; splitting it would obscure that order.
func (c *projectConsistencyCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{Allowed: true}
	cd := c.cd
	if cd == nil || cd.cluster == nil || cd.nutanixClusterConfigSpec == nil {
		return result
	}

	if skip.New(cd.cluster).For(projectConsistencyCheckName) {
		return result
	}

	policy, explicitProjectUUID, err := getActiveProjectPolicy(cd)
	if err != nil {
		field := projectPolicyAnnotationField
		var policyErr *policyValidationError
		if errors.As(err, &policyErr) && policyErr.field != "" {
			field = policyErr.field
		}
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: err.Error(),
			Field:   field,
		})
		return result
	}

	if policy == policySingleProject {
		for _, missing := range collectNutanixMachinesWithNoProjectRef(cd) {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: singleProjectMissingRefMessage(explicitProjectUUID),
				Field:   missing.field,
			})
		}
	}

	refs := collectNutanixProjectRefsWithFields(cd)
	missingRefs := collectNutanixMachinesWithNoProjectRef(cd)
	if len(refs) == 0 {
		switch policy {
		case policySingleProject:
			if result.Allowed {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Project policy %q requires explicit project references but none were found.",
						policy,
					),
					Field: "$.spec.topology.variables",
				})
			}
			return result
		case nutanixprojectpolicy.PolicyUnrestricted, nutanixprojectpolicy.PolicyDefaultOnly:
			return result
		default:
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Unsupported project policy %q encountered during project consistency validation.",
					policy,
				),
				Field: projectPolicyAnnotationField,
			})
			return result
		}
	}
	if cd.nclient == nil {
		failCheckClientNotInitialized(&result, prismCentralEndpointFieldPath, "validate project consistency")
		return result
	}

	resolvedByField := map[string]*resolvedProject{}
	resolutionFailed := false
	for _, ref := range refs {
		resolved, resolveErr := cd.nclient.ResolveProject(ctx, ref.id, cd.pcVersion)
		if resolveErr != nil {
			resolutionFailed = true
			result.Allowed = false
			switch {
			case errors.Is(resolveErr, ErrProjectNotFound):
				result.Causes = append(result.Causes, preflight.Cause{
					Message: "Referenced Nutanix project was not found in Prism Central.",
					Field:   ref.field,
				})
			case errors.Is(resolveErr, ErrProjectNotAccessible):
				result.Causes = append(result.Causes, preflight.Cause{
					Message: "Referenced Nutanix project is not accessible with current credentials.",
					Field:   ref.field,
				})
			default:
				result.InternalError = true
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf("Failed to resolve Nutanix project: %s. Please retry.", resolveErr),
					Field:   ref.field,
				})
			}
			continue
		}
		resolvedByField[ref.field] = resolved
	}

	// If project resolution already failed, skip policy alignment to keep causes focused.
	if resolutionFailed {
		return result
	}

	switch policy {
	case policySingleProject:
		for field, resolved := range resolvedByField {
			if strings.EqualFold(strings.TrimSpace(resolved.UUID), explicitProjectUUID) {
				continue
			}
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: singleProjectPolicyMessage(explicitProjectUUID),
				Field:   field,
			})
		}
		return result
	case nutanixprojectpolicy.PolicyUnrestricted:
		is219, err := webhook.IsNKP219ClusterClass(cd.cluster.Spec.Topology.ClassRef.Name)
		if err != nil {
			cd.log.Error(err, "failed to parse NKP ClusterClass version")
			return result
		}
		if !is219 {
			return result
		}
		if hasMultipleEffectiveProjectIdentities(resolvedByField, missingRefs) {
			result.Allowed = false
			for field := range resolvedByField {
				result.Causes = append(result.Causes, preflight.Cause{
					Message: unrestrictedMultipleProjectsMessage(),
					Field:   field,
				})
			}
			for _, missing := range missingRefs {
				result.Causes = append(result.Causes, preflight.Cause{
					Message: unrestrictedMultipleProjectsMessage(),
					Field:   missing.field,
				})
			}
		}
		return result
	}

	if !nutanixprojectpolicy.PCVersionAtLeast76(cd.pcVersion) {
		for field, resolved := range resolvedByField {
			if IsV3DefaultProjectName(resolved.Name) {
				continue
			}
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: "Project selection violates active policy: default-only policy requires control plane " +
					"and all worker pools to resolve to the Prism Central default project.",
				Field: field,
			})
		}
		return result
	}

	def, err := cd.nclient.ResolvePCDefaultProject(ctx, cd.pcVersion)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Failed to resolve the Prism Central default project: %s. Please retry.", err),
			Field:   "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix.prismCentralEndpoint",
		})
		return result
	}

	for field, resolved := range resolvedByField {
		if resolvedProjectMatchesDefault(resolved, def) {
			continue
		}
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "Project selection violates active policy: default-only policy requires control plane " +
				"and all worker pools to resolve to the Prism Central default project.",
			Field: field,
		})
	}
	return result
}

func collectNutanixProjectRefsWithFields(cd *checkDependencies) []projectRefWithField {
	var out []projectRefWithField
	if cd == nil || cd.nutanixClusterConfigSpec == nil {
		return out
	}
	if cp := cd.nutanixClusterConfigSpec.ControlPlane; cp != nil && cp.Nutanix != nil {
		if p := cp.Nutanix.MachineDetails.Project; p != nil {
			out = append(out, projectRefWithField{
				id:    p,
				field: controlPlaneProjectField,
			})
		}
	}
	if cd.cluster == nil || !cd.cluster.Spec.Topology.IsDefined() {
		return out
	}
	for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
		md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
		ws := cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName[md.Name]
		if ws == nil || ws.Nutanix == nil {
			continue
		}
		if p := ws.Nutanix.MachineDetails.Project; p == nil {
			continue
		}
		out = append(out, projectRefWithField{
			id:    ws.Nutanix.MachineDetails.Project,
			field: workerProjectField(md),
		})
	}
	return out
}

func collectNutanixMachinesWithNoProjectRef(cd *checkDependencies) []projectRefWithField {
	var out []projectRefWithField
	if cd == nil || cd.nutanixClusterConfigSpec == nil {
		return out
	}
	if cp := cd.nutanixClusterConfigSpec.ControlPlane; cp != nil && cp.Nutanix != nil {
		if cp.Nutanix.MachineDetails.Project == nil {
			out = append(out, projectRefWithField{field: controlPlaneProjectField})
		}
	}
	if cd.cluster == nil || !cd.cluster.Spec.Topology.IsDefined() {
		return out
	}
	for i := range cd.cluster.Spec.Topology.Workers.MachineDeployments {
		md := &cd.cluster.Spec.Topology.Workers.MachineDeployments[i]
		ws := cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName[md.Name]
		if ws == nil || ws.Nutanix == nil {
			continue
		}
		if ws.Nutanix.MachineDetails.Project != nil {
			continue
		}
		out = append(out, projectRefWithField{field: workerProjectField(md)})
	}
	return out
}

func workerProjectField(md *clusterv1.MachineDeploymentTopology) string {
	if len(md.Variables.Overrides) > 0 &&
		variables.GetClusterVariableByName(carenv1.WorkerConfigVariableName, md.Variables.Overrides) != nil {
		return fmt.Sprintf(
			"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=='%s'].value.nutanix"+
				".machineDetails.project",
			md.Name,
			carenv1.WorkerConfigVariableName,
		)
	}
	return fmt.Sprintf(
		"$.spec.topology.variables[?@.name=='%s'].value.nutanix.machineDetails.project",
		carenv1.WorkerConfigVariableName,
	)
}

func singleProjectPolicyMessage(projectUUID string) string {
	return fmt.Sprintf(
		"Project policy violation: control plane and each worker pool must resolve to the same "+
			"Prism Central project with uuid %s.",
		projectUUID,
	)
}

func singleProjectMissingRefMessage(projectUUID string) string {
	return singleProjectPolicyMessage(projectUUID) +
		" A missing project reference uses the Prism Central default project."
}

func hasMultipleEffectiveProjectIdentities(
	resolvedByField map[string]*resolvedProject,
	missingRefs []projectRefWithField,
) bool {
	projects := map[string]struct{}{}
	for _, resolved := range resolvedByField {
		projects[resolvedProjectIdentity(resolved)] = struct{}{}
	}
	for range missingRefs {
		projects["default"] = struct{}{}
	}
	return len(projects) > 1
}

func resolvedProjectIdentity(resolved *resolvedProject) string {
	if resolved == nil {
		return ""
	}
	if projectUUID := strings.TrimSpace(resolved.UUID); projectUUID != "" {
		return "uuid:" + strings.ToLower(projectUUID)
	}
	return "name:" + strings.ToLower(strings.TrimSpace(resolved.Name))
}

func unrestrictedMultipleProjectsMessage() string {
	return "Cluster is spread across multiple Prism Central projects. Change the project references in the " +
		"control plane and all MachineDeployments to the same Prism Central project, then retry."
}

func getActiveProjectPolicy(cd *checkDependencies) (policy, singleProjectUUID string, err error) {
	if cd == nil || cd.cluster == nil {
		return "", "", &policyValidationError{
			field:   projectPolicyAnnotationField,
			message: "cluster context is not initialized",
		}
	}
	annotations := cd.cluster.GetAnnotations()
	raw := strings.TrimSpace(annotations[nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey])
	if raw == "" {
		return "", "", &policyValidationError{
			field: projectPolicyAnnotationField,
			message: fmt.Sprintf(
				"project policy annotation %q is missing",
				nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey,
			),
		}
	}
	switch raw {
	case nutanixprojectpolicy.PolicyDefaultOnly, nutanixprojectpolicy.PolicyUnrestricted:
		return raw, "", nil
	case policySingleProject:
		targetUUID := strings.TrimSpace(annotations[singleProjectUUIDAnnotationKey])
		if targetUUID == "" || !isValidUUID(targetUUID) {
			return "", "", &policyValidationError{
				field: singleProjectUUIDAnnotationField,
				message: fmt.Sprintf(
					"single-project policy misconfigured: missing/invalid %s",
					singleProjectUUIDAnnotationKey,
				),
			}
		}
		return raw, targetUUID, nil
	default:
		return "", "", &policyValidationError{
			field:   projectPolicyAnnotationField,
			message: fmt.Sprintf("project policy value %q is not supported by preflight", raw),
		}
	}
}

func isValidUUID(value string) bool {
	_, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil
}

func resolvedProjectMatchesDefault(resolved *resolvedProject, def *PCDefaultProject) bool {
	if resolved == nil || def == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(resolved.UUID), strings.TrimSpace(def.UUID)) ||
		(resolved.Name != "" && def.Name != "" &&
			strings.EqualFold(strings.TrimSpace(resolved.Name), strings.TrimSpace(def.Name)))
}
