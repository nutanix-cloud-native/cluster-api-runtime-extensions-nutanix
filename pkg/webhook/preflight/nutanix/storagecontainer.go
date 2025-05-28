package nutanix

import (
	"context"
	"fmt"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	"k8s.io/utils/ptr"
)

// StorageContainers checks if the storage container specified in the CSIProvider's StorageClassConfigs exists
// in the Nutanix cluster specified in the NutanixNodeSpec.
func (n *Checker) StorageContainers(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Name:    "StorageContainers",
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

	// If the clusterConfig is nil or does not have Addons or CSI, we do not have to check for storage containers.
	if clusterConfig == nil || clusterConfig.Addons == nil || clusterConfig.Addons.CSI == nil {
		return result
	}

	csiProvider := ptr.To(clusterConfig.Addons.CSI.Providers["nutanix"])
	// If the CSIProvider is nil, we cannot check for storage containers.
	if csiProvider == nil {
		return result
	}

	if clusterConfig.ControlPlane != nil && clusterConfig.ControlPlane.Nutanix != nil {
		n.storageContainerCheck(
			ctx,
			clusterConfig.ControlPlane.Nutanix,
			csiProvider,
			"cluster.spec.topology.variables[.name=clusterConfig].controlPlane.nutanix",
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
				n.storageContainerCheck(
					ctx,
					workerConfig.Nutanix,
					csiProvider,
					fmt.Sprintf(
						"workers.machineDeployments[.name=%s].variables.overrides[.name=workerConfig].value.nutanix",
						md.Name,
					),
					&result,
				)
			}
		}
	}

	return result
}

// storageContainerCheck checks if the storage container specified in the CSIProvider's StorageClassConfigs exists.
// It admits the NodeSpec instead of the MachineDetails because the failure domains will be specified in the NodeSpec
// and the MachineDetails.Cluster will be nil in that case.
func (n *Checker) storageContainerCheck(ctx context.Context, nodeSpec *carenv1.NutanixNodeSpec, csiSpec *carenv1.CSIProvider, field string, result *preflight.CheckResult) {
	const (
		csiParameterKeyStorageContainer = "storageContainer"
	)

	if csiSpec == nil {
		result.Allowed = false
		result.Error = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("no storage container found for cluster %q", nodeSpec.MachineDetails.Cluster.Name),
			Field:   field,
		})

		return
	}

	if csiSpec.StorageClassConfigs == nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf("no storage class configs found for cluster %q", nodeSpec.MachineDetails.Cluster.Name),
			Field:   field,
		})

		return
	}

	for _, storageClassConfig := range csiSpec.StorageClassConfigs {
		if storageClassConfig.Parameters == nil {
			continue
		}

		storageContainer, ok := storageClassConfig.Parameters[csiParameterKeyStorageContainer]
		if !ok {
			continue
		}

		// TODO: check if cluster name is set, if not use uuid. If neither is set, use the cluster name from the NodeSpec failure domain.
		if _, err := getStorageContainer(n.nutanixClient, nodeSpec, storageContainer); err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf("failed to check if storage container named %q exists: %s", storageContainer, err),
				Field:   field,
			})

			return
		}
	}
}

func getStorageContainer(client *prismv4.Client, nodeSpec *carenv1.NutanixNodeSpec, storageContainerName string) (*clustermgmtv4.StorageContainer, error) {
	cluster, err := getCluster(client, &nodeSpec.MachineDetails.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	fltr := fmt.Sprintf("name eq '%s' and clusterExtId eq '%s'", storageContainerName, cluster.ExtId)
	resp, err := client.StorageContainerAPI.ListStorageContainers(nil, nil, &fltr, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list storage containers: %w", err)
	}

	containers, ok := resp.GetData().([]clustermgmtv4.StorageContainer)
	if !ok {
		return nil, fmt.Errorf("failed to get data returned by ListStorageContainers(filter=%q)", fltr)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("no storage container named %q found on cluster named %q", storageContainerName, cluster.Name)
	}

	if len(containers) > 1 {
		return nil, fmt.Errorf("multiple storage containers found with name %q on cluster %q", storageContainerName, cluster.Name)
	}

	return ptr.To(containers[0]), nil
}

func getCluster(client *prismv4.Client, clusterIdentifier *v1beta1.NutanixResourceIdentifier) (*clustermgmtv4.Cluster, error) {
	switch clusterIdentifier.Type {
	case v1beta1.NutanixIdentifierUUID:
		resp, err := client.ClustersApiInstance.GetClusterById(clusterIdentifier.UUID)
		if err != nil {
			return nil, err
		}

		cluster, ok := resp.GetData().(clustermgmtv4.Cluster)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetClusterById")
		}

		return &cluster, nil
	case v1beta1.NutanixIdentifierName:
		filter := fmt.Sprintf("name eq '%s'", *clusterIdentifier.Name)
		resp, err := client.ClustersApiInstance.ListClusters(nil, nil, &filter, nil, nil, nil)
		if err != nil {
			return nil, err
		}

		if resp == nil || resp.GetData() == nil {
			return nil, fmt.Errorf("no clusters were returned")
		}

		clusters, ok := resp.GetData().([]clustermgmtv4.Cluster)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListClusters")
		}

		if len(clusters) == 0 {
			return nil, fmt.Errorf("no clusters found with name %q", *clusterIdentifier.Name)
		}

		if len(clusters) > 1 {
			return nil, fmt.Errorf("multiple clusters found with name %q", *clusterIdentifier.Name)
		}

		return &clusters[0], nil
	default:
		return nil, fmt.Errorf("cluster identifier is missing both name and uuid")
	}
}
