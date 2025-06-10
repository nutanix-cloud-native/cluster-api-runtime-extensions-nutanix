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

func initVMImageChecks(
	n *nutanixChecker,
) []preflight.Check {
	checks := []preflight.Check{}

	if n.optOut.For("NutanixVMImage") {
		n.log.V(5).Info("Opted out of Nutanix VM image checks")
		return checks
	}

	if n.nutanixClusterConfigSpec != nil && n.nutanixClusterConfigSpec.ControlPlane != nil &&
		n.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks,
			n.vmImageCheckFunc(
				n,
				&n.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
				"cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix.machineDetails",
			),
		)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range n.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				n.vmImageCheckFunc(
					n,
					&nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails,
					fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s]"+
							".variables[.name=workerConfig].value.nutanix.machineDetails",
						mdName,
					),
				),
			)
		}
	}

	return checks
}

func vmImageCheck(
	n *nutanixChecker,
	machineDetails *carenv1.NutanixMachineDetails,
	field string,
) preflight.Check {
	n.log.V(5).Info("Initializing Nutanix VM image check", "field", field)

	return func(ctx context.Context) preflight.CheckResult {
		result := preflight.CheckResult{
			Name:    "NutanixVMImage",
			Allowed: false,
		}

		// If the v4 client is not initialized, we cannot perform VM image checks.
		if n.v4client == nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes,
				preflight.Cause{
					Message: "Nutanix v4 client is not initialized, cannot perform VM image checks",
					Field:   "",
				},
			)
			return result
		}

		if machineDetails.ImageLookup != nil {
			result.Allowed = true
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf("%s uses imageLookup, which is not yet supported by checks", field),
			)
			return result
		}

		if machineDetails.Image != nil {
			imagesCh := make(chan []vmmv4.Image)
			defer close(imagesCh)
			errCh := make(chan error)
			defer close(errCh)

			images, err := getVMImages(n.v4client, machineDetails.Image)
			if err != nil {
				result.Allowed = false
				result.Error = true
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf("failed to get VM Image: %s", err),
					Field:   field,
				})
				return result
			}

			if len(images) != 1 {
				result.Allowed = false
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf("expected to find 1 VM Image, found %d", len(images)),
					Field:   field,
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
}

func getVMImages(
	client v4client,
	id *capxv1.NutanixResourceIdentifier,
) ([]vmmv4.Image, error) {
	switch {
	case id.IsUUID():
		resp, err := client.GetImageById(id.UUID)
		if err != nil {
			return nil, err
		}
		image, ok := resp.GetData().(vmmv4.Image)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetImageById")
		}
		return []vmmv4.Image{image}, nil
	case id.IsName():
		filter_ := fmt.Sprintf("name eq '%s'", *id.Name)
		resp, err := client.ListImages(nil, nil, &filter_, nil, nil)
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
