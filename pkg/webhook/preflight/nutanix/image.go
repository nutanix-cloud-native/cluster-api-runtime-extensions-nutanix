package nutanix

import (
	"context"
	"fmt"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func (n *Checker) VMImages(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Name:    "VMImages",
		Allowed: true,
	}

	// Check control plane VM image.
	clusterConfig, err := n.variablesGetter.ClusterConfig()
	if err != nil {
		result.Error = true
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("failed to read clusterConfig variable: %s", err),
			Field:   "cluster.spec.topology.variables",
		})
	}
	if clusterConfig != nil && clusterConfig.ControlPlane != nil && clusterConfig.ControlPlane.Nutanix != nil {
		n.vmImageCheckForMachineDetails(
			ctx,
			clusterConfig,
			&clusterConfig.ControlPlane.Nutanix.MachineDetails,
			"cluster.spec.topology.variables[.name=clusterConfig].controlPlane.nutanix.machineDetails",
			&result,
		)
	}

	// Check worker VM images.
	if n.cluster.Spec.Topology.Workers != nil {
		for _, md := range n.cluster.Spec.Topology.Workers.MachineDeployments {
			workerConfig, err := n.variablesGetter.WorkerConfigForMachineDeployment(md)
			if err != nil {
				result.Error = true
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf("failed to read workerConfig variable: %s", err),
					Field: fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s].variables.overrides",
						md.Name,
					),
				})
			}
			if workerConfig != nil && workerConfig.Nutanix != nil {
				n.vmImageCheckForMachineDetails(
					ctx,
					clusterConfig,
					&workerConfig.Nutanix.MachineDetails,
					fmt.Sprintf(
						"workers.machineDeployments[.name=%s].variables.overrides[.name=workerConfig].value.nutanix.machineDetails",
						md.Name,
					),
					&result,
				)
			}
		}
	}

	return result
}

func (n *Checker) vmImageCheckForMachineDetails(
	ctx context.Context,
	clusterConfig *variables.ClusterConfigSpec,
	details *carenv1.NutanixMachineDetails,
	field string,
	result *preflight.CheckResult,
) {
	if details.ImageLookup != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "ImageLookup is not yet supported",
			Field:   field,
		})
		return
	}

	if details.Image != nil {
		client, err := n.clientGetter.V4(ctx, clusterConfig)
		if err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf("failed to get Nutanix client: %s", err),
				Field:   field,
			})
			return
		}

		images, err := getVMImages(client, details.Image)
		if err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf("failed to count matching VM Images: %s", err),
				Field:   field,
			})
			return
		}

		if len(images) != 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf("expected to find 1 VM Image, found %d", len(images)),
				Field:   field,
			})
		}
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
