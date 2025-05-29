// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func newVMImageCheck(
	client *prismv4.Client,
	nutanixNodeSpec *carenv1.NutanixNodeSpec,
	nutanixNodeSpecField string,
) preflight.Check {
	if nutanixNodeSpec == nil {
		return func(ctx context.Context) preflight.CheckResult {
			return preflight.CheckResult{
				Name:    "VMImage",
				Allowed: false,
				Error:   true,
				Causes: []preflight.Cause{
					{
						Message: "NutanixNodeSpec is missing",
						Field:   nutanixNodeSpecField,
					},
				},
			}
		}
	}

	return func(ctx context.Context) preflight.CheckResult {
		return vmImageCheckForMachineDetails(
			client,
			&nutanixNodeSpec.MachineDetails,
			fmt.Sprintf("%s.machineDetails", nutanixNodeSpecField),
		)
	}
}

func vmImageCheckForMachineDetails(
	client *prismv4.Client,
	details *carenv1.NutanixMachineDetails,
	field string,
) preflight.CheckResult {
	result := preflight.CheckResult{
		Name:    "VMImage",
		Allowed: false,
	}
	if details.ImageLookup != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "ImageLookup is not yet supported",
			Field:   field,
		})
		return result
	}

	if details.Image != nil {
		imagesCh := make(chan []vmmv4.Image)
		defer close(imagesCh)
		errCh := make(chan error)
		defer close(errCh)

		images, err := getVMImages(client, details.Image)
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

func getVMImages(
	client *prismv4.Client,
	id *capxv1.NutanixResourceIdentifier,
) ([]vmmv4.Image, error) {
	switch {
	case id.IsUUID():
		resp, err := client.ImagesApiInstance.GetImageById(id.UUID)
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
		resp, err := client.ImagesApiInstance.ListImages(nil, nil, &filter_, nil, nil)
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
