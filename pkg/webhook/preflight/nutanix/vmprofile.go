// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	vmmv4ahv "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config" ///nolint:lll // Import path is long.

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1" ///nolint:lll // Import path is long.
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type vmProfileCheck struct {
	machineDetails *carenv1.NutanixMachineDetails
	field          string
	nclient        client
	pcVersion      string
	projectRef     *capxv1.NutanixResourceIdentifier
}

func (c *vmProfileCheck) Name() string {
	return "NutanixVMProfile"
}

func (c *vmProfileCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	// If VMProfile is not set, there's nothing to validate
	if c.machineDetails.VMProfile == nil {
		return result
	}

	scopeCtx, err := resolveProjectScopeContext(ctx, c.nclient, c.pcVersion, c.projectRef)
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("Failed to resolve project scope for VM Profile validation: %s. Please retry.", err),
			Field:   c.field + ".project",
		})
		return result
	}

	var vmProfiles []vmmv4ahv.VmProfile
	if scopeCtx != nil {
		vmProfiles, err = getVMProfilesInProject(ctx, c.nclient, c.machineDetails.VMProfile, scopeCtx.projectUUID)
	} else {
		vmProfiles, err = getVMProfiles(ctx, c.nclient, c.machineDetails.VMProfile)
	}
	if err != nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Failed to get VM Profile %q: %s. This is usually a temporary error. Please retry.", ///nolint:lll // Message is long.
				c.machineDetails.VMProfile,
				err,
			),
			Field: c.field + ".vmProfile",
		})
		return result
	}

	switch len(vmProfiles) {
	case 0:
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"No VM Profile found in Prism Central that matches identifier %q. Please contact your Prism Central administrator to ensure the VM Profile exists. Once the VM Profile is available in Prism Central, retry.", ///nolint:lll // Message is long.
				c.machineDetails.VMProfile,
			),
			Field: c.field + ".vmProfile",
		})
		return result
	case 1:
		// Check if the VM Profile has NIC profiles configured
		vmProfile := vmProfiles[0]
		hasNicProfiles := false
		if vmProfile.VmConfiguration != nil && vmProfile.VmConfiguration.Nics != nil {
			// Check if at least one NIC has a non-empty ExtId
			for _, nic := range vmProfile.VmConfiguration.Nics {
				if nic.ExtId != nil && *nic.ExtId != "" {
					hasNicProfiles = true
					break
				}
			}
		}
		if !hasNicProfiles {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"The VMProfile chosen (%q) has not been configured with NIC profiles. Please contact your Prism Central administrator to configure NIC profiles for this VM Profile, then retry.", ///nolint:lll // Message is long.
					c.machineDetails.VMProfile,
				),
				Field: c.field + ".vmProfile",
			})
			return result
		}
		result.Allowed = true
		return result
	default: // Found multiple profiles.
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"Found %d VM Profiles in Prism Central that match name %q. There must be exactly 1 VM Profile that matches this name. Please contact your Prism Central administrator to resolve duplicate VM Profiles, use a different VM Profile, or identify the VM Profile by its UUID, then retry.", ///nolint:lll // Message is long.
				len(vmProfiles),
				c.machineDetails.VMProfile,
			),
			Field: c.field + ".vmProfile",
		})
		return result
	}
}

func newVMProfileChecks(
	cd *checkDependencies,
) []preflight.Check {
	checks := []preflight.Check{}

	if cd == nil || cd.nclient == nil || cd.pcVersion == "" {
		return checks
	}

	// Check control plane VM profile
	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks,
			&vmProfileCheck{
				machineDetails: &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
				field:          controlPlaneMachineDetailsField,
				nclient:        cd.nclient,
				pcVersion:      cd.pcVersion,
				projectRef:     cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails.Project,
			},
		)
	}

	// Check worker node VM profiles
	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName { ///nolint:lll // Variable name is long.
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				&vmProfileCheck{
					machineDetails: &nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails,
					field: fmt.Sprintf(
						"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=='%s'].value.nutanix.machineDetails", //nolint:lll // Field is long.
						mdName,
						carenv1.WorkerConfigVariableName,
					),
					nclient:    cd.nclient,
					pcVersion:  cd.pcVersion,
					projectRef: nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails.Project,
				},
			)
		}
	}

	return checks
}

func getVMProfilesInProject(
	ctx context.Context,
	client client,
	id *capxv1.NutanixResourceIdentifier,
	projectUUID string,
) ([]vmmv4ahv.VmProfile, error) {
	projectUUID = escapeODataString(projectUUID)
	switch {
	case id.IsUUID():
		filter := fmt.Sprintf(
			"extId eq '%s' and (projectExtId eq '%s' or sharedWithProjects/any(a:a eq '%s'))",
			escapeODataString(*id.UUID),
			projectUUID,
			projectUUID,
		)
		return client.ListVMProfilesByFilter(ctx, filter)
	case id.IsName():
		filter := fmt.Sprintf(
			"name eq '%s' and (projectExtId eq '%s' or sharedWithProjects/any(a:a eq '%s'))",
			escapeODataString(*id.Name),
			projectUUID,
			projectUUID,
		)
		return client.ListVMProfilesByFilter(ctx, filter)
	default:
		return nil, fmt.Errorf("vm profile identifier is missing both name and uuid")
	}
}

func getVMProfiles(
	ctx context.Context,
	client client,
	id *capxv1.NutanixResourceIdentifier,
) ([]vmmv4ahv.VmProfile, error) {
	switch {
	case id.IsUUID():
		resp, err := client.GetVMProfileById(ctx, id.UUID)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			// No VM profiles were returned.
			return []vmmv4ahv.VmProfile{}, nil
		}
		vmProfile, ok := resp.GetData().(vmmv4ahv.VmProfile)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetVMProfileById")
		}
		return []vmmv4ahv.VmProfile{vmProfile}, nil
	case id.IsName():
		filter_ := fmt.Sprintf("name eq '%s'", *id.Name)
		return client.ListVMProfilesByFilter(ctx, filter_)
	default:
		return nil, fmt.Errorf("vm profile identifier is missing both name and uuid")
	}
}
