// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"strings"

	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

// subnetCheck validates that a single machineDetails.subnets entry resolves to exactly one subnet that is
// reachable from the machine's Prism Element cluster and (on PC >= 7.6) usable by the machine's project.
//
// This mirrors CAPX's controllers.GetSubnetUUID so that misconfigured subnets fail at admission instead of
// later during NutanixMachine reconcile (e.g. "failed to retrieve subnet by name <name> in project <name>").
// Subnets referenced via a failure domain are validated by the NutanixFailureDomain check, so this check
// only runs for machineDetails that directly reference a Prism Element cluster.
type subnetCheck struct {
	subnetID     capxv1.NutanixResourceIdentifier
	clusterRef   *capxv1.NutanixResourceIdentifier
	field        string
	projectField string
	nclient      client
	pcVersion    string
	projectRef   *capxv1.NutanixResourceIdentifier
}

func (c *subnetCheck) Name() string {
	return "NutanixSubnet"
}

func (c *subnetCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	scopeCtx, err := resolveProjectScopeContext(ctx, c.nclient, c.pcVersion, c.projectRef)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Failed to resolve project scope for subnet validation: %s. Please retry.", err),
			Field:   c.projectField,
		})
		return result
	}

	var subnets []netv4.Subnet
	if scopeCtx != nil {
		subnets, err = getSubnetsInProject(ctx, c.nclient, &c.subnetID, scopeCtx.projectUUID)
	} else {
		subnets, err = getSubnets(ctx, c.nclient, &c.subnetID)
	}
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to get subnet %q: %s. This is usually a temporary error. Please retry.",
				c.subnetID,
				err,
			),
			Field: c.field,
		})
		return result
	}

	// Scope the matches to the machine's Prism Element cluster (CAPX GetSubnetUUID): overlay by SubnetType,
	// otherwise ClusterReference and/or ClusterReferenceList must name the PE.
	peUUID, ok := c.resolvePrismElementUUID(ctx, scopeCtx, &result)
	if !ok {
		return result
	}

	filteredSubnets := filterSubnetsForPrismElement(subnets, peUUID)
	// Prefer subnets owned by the project over those merely shared with it, matching the VM Image and
	// VM Profile checks. This is applied only as a tie-breaker after Prism Element narrowing, so a valid
	// shared subnet on the machine's Prism Element is never discarded in favour of an owned subnet on a
	// different Prism Element.
	if len(filteredSubnets) > 1 && scopeCtx != nil {
		filteredSubnets = preferProjectOwnedSubnets(filteredSubnets, scopeCtx.projectUUID)
	}

	if len(filteredSubnets) != 1 {
		result.Allowed = false
		// Name the Prism Element cluster (always set for this check). The project clause is included only
		// when the lookup ran project-scoped (getSubnetsInProject filters by project ownership/sharing);
		// for non-project-scoped lookups (PC < 7.6) no project filtering was applied, so it is omitted.
		var message string
		if scopeCtx != nil {
			message = fmt.Sprintf(
				"Found %d Subnets in Prism Central that match identifier %q. There must be exactly 1 Subnet that is attached to the Prism Element cluster %q and is owned by or shared with the referenced Prism Central project %q. Check that the subnet exists and satisfies both conditions, then retry.", //nolint:lll // Message is long.
				len(filteredSubnets),
				c.subnetID,
				c.clusterRef,
				c.projectRef,
			)
		} else {
			message = fmt.Sprintf(
				"Found %d Subnets in Prism Central that match identifier %q. There must be exactly 1 Subnet that is attached to the Prism Element cluster %q. Check that the subnet exists and is attached to that Prism Element cluster, then retry.", //nolint:lll // Message is long.
				len(filteredSubnets),
				c.subnetID,
				c.clusterRef,
			)
		}
		result.Causes = append(result.Causes, preflight.Cause{
			Message: message,
			Field:   c.field,
		})
		return result
	}

	return result
}

// resolvePrismElementUUID resolves the machine's Prism Element cluster reference to a single UUID used to
// scope subnet matches. It returns ("", true) when no cluster is referenced (PE scoping is then skipped).
// On resolution failure it records a cause on result and returns ("", false).
//
// A UUID reference is used directly, avoiding a cluster-management API call entirely. A name reference is
// resolved via the project's resource groups when the user is project-scoped (the global cluster-management
// APIs used by getClusters are typically forbidden for project-scoped users), and via getClusters otherwise.
func (c *subnetCheck) resolvePrismElementUUID(
	ctx context.Context,
	scopeCtx *projectScopeContext,
	result *preflight.CheckResult,
) (string, bool) {
	if c.clusterRef == nil || (!c.clusterRef.IsUUID() && !c.clusterRef.IsName()) {
		return "", true
	}

	if c.clusterRef.IsUUID() && c.clusterRef.UUID != nil {
		return strings.TrimSpace(*c.clusterRef.UUID), true
	}

	if scopeCtx != nil && scopeCtx.scope == userScopeProjectScoped {
		peUUID, err := resolvePrismElementUUIDInProject(ctx, c.nclient, scopeCtx.projectUUID, c.clusterRef)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to resolve the Prism Element cluster %q referenced alongside subnet %q within the project scope: %s. Please retry.", //nolint:lll // Message is long.
					c.clusterRef,
					c.subnetID,
					err,
				),
				Field: c.field,
			})
			return "", false
		}
		if peUUID == "" {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Prism Element cluster %q referenced alongside subnet %q is not available in the selected project scope. Please contact your Prism Central administrator to grant the project access to this Prism Element cluster, then retry.", //nolint:lll // Message is long.
					c.clusterRef,
					c.subnetID,
				),
				Field: c.field,
			})
			return "", false
		}
		return peUUID, true
	}

	peClusters, err := getClusters(ctx, c.nclient, c.clusterRef)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to check the Prism Element cluster %q referenced alongside subnet %q: %s. This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
				c.clusterRef,
				c.subnetID,
				err,
			),
			Field: c.field,
		})
		return "", false
	}
	if len(peClusters) != 1 || peClusters[0].ExtId == nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Found %d Prism Element cluster(s) that match identifier %q referenced alongside subnet %q. There must be exactly 1 Prism Element cluster that matches this identifier.", //nolint:lll // Message is long.
				len(peClusters),
				c.clusterRef,
				c.subnetID,
			),
			Field: c.field,
		})
		return "", false
	}

	return strings.TrimSpace(*peClusters[0].ExtId), true
}

// newSubnetChecks builds a NutanixSubnet check for every subnet referenced directly by a control plane or
// worker machineDetails (i.e. not via a failure domain, which is validated by the NutanixFailureDomain
// check).
func newSubnetChecks(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return checks
	}

	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		machineDetails := &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails
		machineDetailsField := controlPlaneMachineDetailsField
		checks = append(checks, subnetChecksForMachineDetails(
			cd,
			machineDetails,
			machineDetailsField,
		)...)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec == nil || nutanixWorkerNodeConfigSpec.Nutanix == nil {
			continue
		}
		machineDetails := &nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails
		machineDetailsField := fmt.Sprintf(
			"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=='%s'].value.nutanix.machineDetails", //nolint:lll // Field is long.
			mdName,
			carenv1.WorkerConfigVariableName,
		)
		checks = append(checks, subnetChecksForMachineDetails(
			cd,
			machineDetails,
			machineDetailsField,
		)...)
	}

	return checks
}

// preferProjectOwnedSubnets returns only the subnets owned by the given project (projectExtId matches) when
// at least one is owned; otherwise it returns the input unchanged (all matches are shared with the project).
func preferProjectOwnedSubnets(subnets []netv4.Subnet, projectUUID string) []netv4.Subnet {
	owned := make([]netv4.Subnet, 0, len(subnets))
	for i := range subnets {
		if subnets[i].ProjectExtId != nil &&
			strings.EqualFold(strings.TrimSpace(*subnets[i].ProjectExtId), strings.TrimSpace(projectUUID)) {
			owned = append(owned, subnets[i])
		}
	}
	if len(owned) > 0 {
		return owned
	}
	return subnets
}

func subnetChecksForMachineDetails(
	cd *checkDependencies,
	machineDetails *carenv1.NutanixMachineDetails,
	machineDetailsField string,
) []preflight.Check {
	// Subnets referenced without a Prism Element cluster come from a failure domain and are validated by the
	// NutanixFailureDomain check instead.
	if machineDetails.Cluster == nil || (!machineDetails.Cluster.IsUUID() && !machineDetails.Cluster.IsName()) {
		return nil
	}

	projectField := machineDetailsField + ".project"
	checks := make([]preflight.Check, 0, len(machineDetails.Subnets))
	for i := range machineDetails.Subnets {
		checks = append(checks, &subnetCheck{
			subnetID:     machineDetails.Subnets[i],
			clusterRef:   machineDetails.Cluster,
			field:        fmt.Sprintf("%s.subnets[%d]", machineDetailsField, i),
			projectField: projectField,
			nclient:      cd.nclient,
			pcVersion:    cd.pcVersion,
			projectRef:   machineDetails.Project,
		})
	}
	return checks
}
