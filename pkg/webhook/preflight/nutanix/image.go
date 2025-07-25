// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

type imageCheck struct {
	machineDetails *carenv1.NutanixMachineDetails
	field          string
	nclient        client
}

func (c *imageCheck) Name() string {
	return "NutanixVMImage"
}

func (c *imageCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: false,
	}

	if c.machineDetails.ImageLookup != nil {
		result.Allowed = true
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf("Field %s uses imageLookup, which is not yet supported by checks", c.field),
		)
		return result
	}

	if c.machineDetails.Image != nil {
		images, err := getVMImages(ctx, c.nclient, c.machineDetails.Image)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to get VM Image %q: %s. This is usually a temporary error. Please retry.",
					c.machineDetails.Image,
					err,
				),
				Field: c.field + ".image",
			})
			return result
		}

		if len(images) != 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found %d VM Images in Prism Central that match identifier %q. There must be exactly 1 VM Image that matches this identifier. Remove duplicate VM Images, use a different VM Image, or identify the VM Image by its UUID, then retry.", ///nolint:lll // Message is long.
					len(images),
					c.machineDetails.Image,
				),
				Field: c.field + ".image",
			})
			return result
		}

		// Found exactly one image.
		result.Allowed = true
		return result
	}

	// Neither ImageLookup nor Image is specified.
	return result
}

func newVMImageChecks(
	cd *checkDependencies,
) []preflight.Check {
	checks := []preflight.Check{}

	if cd.nclient == nil {
		return checks
	}

	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks,
			&imageCheck{
				machineDetails: &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
				field:          "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix.controlPlane.machineDetails", ///nolint:lll // Field is long.
				nclient:        cd.nclient,
			},
		)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				&imageCheck{
					machineDetails: &nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails,
					///nolint:lll // The field is long.
					field: fmt.Sprintf(
						"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=workerConfig].value.nutanix.machineDetails",
						mdName,
					),
					nclient: cd.nclient,
				},
			)
		}
	}

	return checks
}

func getVMImages(
	ctx context.Context,
	client client,
	id *capxv1.NutanixResourceIdentifier,
) ([]vmmv4.Image, error) {
	switch {
	case id.IsUUID():
		resp, err := client.GetImageById(ctx, id.UUID)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			// No images were returned.
			return []vmmv4.Image{}, nil
		}
		image, ok := resp.GetData().(vmmv4.Image)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetImageById")
		}
		return []vmmv4.Image{image}, nil
	case id.IsName():
		filter_ := fmt.Sprintf("name eq '%s'", *id.Name)
		resp, err := client.ListImages(ctx, nil, nil, &filter_, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.GetData() == nil {
			// No images were returned.
			return []vmmv4.Image{}, nil
		}
		images, ok := resp.GetData().([]vmmv4.Image)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListImages")
		}
		return images, nil
	default:
		return nil, fmt.Errorf("image identifier is missing both name and uuid")
	}
}
