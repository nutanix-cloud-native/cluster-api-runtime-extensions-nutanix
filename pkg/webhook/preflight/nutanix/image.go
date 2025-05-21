package nutanix

import (
	"context"
	"fmt"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	vmmv4 "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func (n *Checker) VMImageCheck(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	// Check control plane VM image.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(n.cluster.Spec.Topology.Variables)
	if err != nil {
		result.Error = true
		result.Causes = append(result.Causes, metav1.StatusCause{
			Type: "VMImageCheck",
			Message: fmt.Sprintf(
				"failed to unmarshal topology variable %q: %s",
				carenv1.ClusterConfigVariableName,
				err,
			),
			Field: "cluster.spec.topology.variables",
		})
	} else if clusterConfig != nil {
		if clusterConfig.ControlPlane == nil || clusterConfig.ControlPlane.Nutanix == nil {
			result.Causes = append(result.Causes, metav1.StatusCause{
				Type:    "VMImageCheck",
				Message: "missing Nutanix configuration in cluster topology",
				Field:   "cluster.spec.topology.controlPlane.nutanix",
			})
		}

		n.vmImageCheckForMachineDetails(
			ctx,
			&clusterConfig.ControlPlane.Nutanix.MachineDetails,
			"controlPlane.nutanix.machineDetails",
			&result,
		)
	}

	// Check worker VM images.
	if n.cluster.Spec.Topology.Workers == nil {
		return result
	}

	for _, md := range n.cluster.Spec.Topology.Workers.MachineDeployments {
		if md.Variables == nil {
			continue
		}

		workerConfig, err := variables.UnmarshalWorkerConfigVariable(md.Variables.Overrides)
		if err != nil {
			result.Error = true
			result.Causes = append(result.Causes, metav1.StatusCause{
				Type: "VMImageCheck",
				Message: fmt.Sprintf(
					"failed to unmarshal topology variable %q: %s",
					carenv1.WorkerConfigVariableName,
					err,
				),
				Field: fmt.Sprintf(
					"cluster.spec.topology.workers.machineDeployments[.name=%s].variables.overrides",
					md.Name,
				),
			})
		} else if workerConfig != nil {
			if workerConfig.Nutanix == nil {
				result.Causes = append(result.Causes, metav1.StatusCause{
					Type:    "VMImageCheck",
					Message: "missing Nutanix configuration in worker machine deployment",
					Field: fmt.Sprintf("cluster.spec.topology.workers.machineDeployments[.name=%s]"+
						".variables.overrides[.name=workerConfig].value.nutanix", md.Name),
				})
			} else {
				n.vmImageCheckForMachineDetails(
					ctx,
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
	details *carenv1.NutanixMachineDetails,
	field string,
	result *preflight.CheckResult,
) {
	if details.ImageLookup != nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes, metav1.StatusCause{
			Type:    "VMImageCheck",
			Message: "ImageLookup is not yet supported",
			Field:   field,
		})
		return
	}

	if details.Image != nil {
		client, err := n.v4client(ctx, n.client, n.cluster.Namespace)
		if err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, metav1.StatusCause{
				Type:    "VMImageCheck",
				Message: fmt.Sprintf("failed to get Nutanix client: %s", err),
				Field:   field,
			})
			return
		}

		images, err := getVMImages(client, details.Image)
		if err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, metav1.StatusCause{
				Type:    "VMImageCheck",
				Message: fmt.Sprintf("failed to count matching VM Images: %s", err),
				Field:   field,
			})
			return
		}

		if len(images) != 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, metav1.StatusCause{
				Type:    "VMImageCheck",
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
