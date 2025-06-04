// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	prismv4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func (n *nutanixChecker) initStorageContainerChecks() []preflight.Check {
	checks := []preflight.Check{}

	if n.nutanixClusterConfigSpec != nil && n.nutanixClusterConfigSpec.ControlPlane != nil &&
		n.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks,
			n.storageContainerCheck(
				n.nutanixClusterConfigSpec.ControlPlane.Nutanix,
				"cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix",
				&n.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
			),
		)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range n.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				n.storageContainerCheck(
					nutanixWorkerNodeConfigSpec.Nutanix,
					fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s]"+
							".variables[.name=workerConfig].value.nutanix",
						mdName,
					),
					&n.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
				),
			)
		}
	}

	return checks
}

// storageContainerCheck checks if the storage container specified in the CSIProvider's StorageClassConfigs exists.
// It admits the NodeSpec instead of the MachineDetails because the failure domains will be specified in the NodeSpec
// and the MachineDetails.Cluster will be nil in that case.
func (n *nutanixChecker) storageContainerCheck(
	nodeSpec *carenv1.NutanixNodeSpec,
	field string,
	csiSpec *carenv1.CSIProvider,
) preflight.Check {
	const (
		csiParameterKeyStorageContainer = "storageContainer"
	)

	return func(ctx context.Context) preflight.CheckResult {
		result := &preflight.CheckResult{}
		if csiSpec == nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"no storage container found for cluster %q",
					*nodeSpec.MachineDetails.Cluster.Name,
				),
				Field: field,
			})

			return *result
		}

		if csiSpec.StorageClassConfigs == nil {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"no storage class configs found for cluster %q",
					*nodeSpec.MachineDetails.Cluster.Name,
				),
				Field: field,
			})

			return *result
		}

		for _, storageClassConfig := range csiSpec.StorageClassConfigs {
			if storageClassConfig.Parameters == nil {
				continue
			}

			storageContainer, ok := storageClassConfig.Parameters[csiParameterKeyStorageContainer]
			if !ok {
				continue
			}

			// TODO: check if cluster name is set, if not use uuid.
			// If neither is set, use the cluster name from the NodeSpec failure domain.
			if _, err := getStorageContainer(n.v4client, nodeSpec, storageContainer); err != nil {
				result.Allowed = false
				result.Error = true
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"failed to check if storage container named %q exists: %s",
						storageContainer,
						err,
					),
					Field: field,
				})

				return *result
			}
		}

		return *result
	}
}

func getStorageContainer(
	client *prismv4.Client,
	nodeSpec *carenv1.NutanixNodeSpec,
	storageContainerName string,
) (*clustermgmtv4.StorageContainer, error) {
	cluster, err := getCluster(client, &nodeSpec.MachineDetails.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	fltr := fmt.Sprintf("name eq %q and clusterExtId eq %q", storageContainerName, *cluster.ExtId)
	resp, err := client.StorageContainerAPI.ListStorageContainers(nil, nil, &fltr, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list storage containers: %w", err)
	}

	containers, ok := resp.GetData().([]clustermgmtv4.StorageContainer)
	if !ok {
		return nil, fmt.Errorf("failed to get data returned by ListStorageContainers(filter=%q)", fltr)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf(
			"no storage container named %q found on cluster named %q",
			storageContainerName,
			*cluster.Name,
		)
	}

	if len(containers) > 1 {
		return nil, fmt.Errorf(
			"multiple storage containers found with name %q on cluster %q",
			storageContainerName,
			*cluster.Name,
		)
	}

	return ptr.To(containers[0]), nil
}

func getCluster(
	client *prismv4.Client,
	clusterIdentifier *v1beta1.NutanixResourceIdentifier,
) (*clustermgmtv4.Cluster, error) {
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
