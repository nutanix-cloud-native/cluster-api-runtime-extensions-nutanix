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

func (n *Checker) VMImageCheck(details carenv1.NutanixMachineDetails, field string) preflight.Check {
	return func(ctx context.Context) preflight.CheckResult {
		result := preflight.CheckResult{
			Allowed: true,
			Field:   field,
		}

		if details.ImageLookup != nil {
			result.Allowed = false
			result.Message = "ImageLookup is not yet supported"
			return result
		}

		if details.Image != nil {
			images, err := getVMImages(n.nutanixClient, details.Image)
			if err != nil {
				result.Allowed = false
				result.Error = true
				result.Message = fmt.Sprintf("failed to count matching VM Images: %s", err)
				return result
			}

			if len(images) != 1 {
				result.Allowed = false
				result.Message = fmt.Sprintf("expected to find 1 VM Image, found %d", len(images))
				return result
			}
		}

		return result
	}
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
