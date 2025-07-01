// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"sync"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	csiParameterKeyStorageContainer = "storageContainer"
)

type storageContainerCheck struct {
	machineSpec *carenv1.NutanixMachineDetails
	field       string
	csiSpec     *carenv1.CSIProvider
	nclient     client
}

func (c *storageContainerCheck) Name() string {
	return "NutanixStorageContainer"
}

func (c *storageContainerCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	if c.csiSpec == nil {
		result.Allowed = false
		result.InternalError = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"no storage container found for cluster %q",
				*c.machineSpec.Cluster.Name,
			),
			Field: c.field,
		})

		return result
	}

	if c.csiSpec.StorageClassConfigs == nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"no storage class configs found for cluster %q",
				*c.machineSpec.Cluster.Name,
			),
			Field: c.field,
		})

		return result
	}

	clusterIdentifier := &c.machineSpec.Cluster

	// We wait to get the cluster until we know we need to.
	// We should only get the cluster once.
	getClusterOnce := sync.OnceValues(func() (*clustermgmtv4.Cluster, error) {
		return getCluster(c.nclient, clusterIdentifier)
	})

	for _, storageClassConfig := range c.csiSpec.StorageClassConfigs {
		if storageClassConfig.Parameters == nil {
			continue
		}

		storageContainer, ok := storageClassConfig.Parameters[csiParameterKeyStorageContainer]
		if !ok {
			continue
		}

		cluster, err := getClusterOnce()
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"failed to check if storage container %q exists: failed to get cluster %q: %s",
					storageContainer,
					clusterIdentifier,
					err,
				),
				Field: c.field,
			})
			continue
		}

		containers, err := getStorageContainers(c.nclient, *cluster.ExtId, storageContainer)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"failed to check if storage container %q exists in cluster %q: %s",
					storageContainer,
					clusterIdentifier,
					err,
				),
				Field: c.field,
			})
			continue
		}

		if len(containers) == 0 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"storage container %q not found on cluster %q",
					storageContainer,
					clusterIdentifier,
				),
				Field: c.field,
			})
			continue
		}

		if len(containers) > 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"multiple storage containers named %q found on cluster %q",
					storageContainer,
					clusterIdentifier,
				),
				Field: c.field,
			})
			continue
		}
	}

	return result
}

func newStorageContainerChecks(cd *checkDependencies) []preflight.Check {
	checks := []preflight.Check{}

	if cd.nclient == nil {
		return checks
	}

	// If there is no CSI configuration, there is no need to check for storage containers.
	if cd.nutanixClusterConfigSpec == nil ||
		cd.nutanixClusterConfigSpec.Addons == nil ||
		cd.nutanixClusterConfigSpec.Addons.CSI == nil {
		return checks
	}

	if cd.nutanixClusterConfigSpec != nil && cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
		checks = append(checks,
			&storageContainerCheck{
				machineSpec: &cd.nutanixClusterConfigSpec.ControlPlane.Nutanix.MachineDetails,
				field:       "cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix.machineDetails",
				csiSpec:     &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
				nclient:     cd.nclient,
			},
		)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				&storageContainerCheck{
					machineSpec: &nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails,
					field: fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s]"+
							".variables[.name=workerConfig].value.nutanix.machineDetails",
						mdName,
					),
					csiSpec: &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
					nclient: cd.nclient,
				},
			)
		}
	}

	return checks
}

func getStorageContainers(
	client client,
	clusterUUID string,
	storageContainerName string,
) ([]clustermgmtv4.StorageContainer, error) {
	fltr := fmt.Sprintf("name eq '%s' and clusterExtId eq '%s'", storageContainerName, clusterUUID)
	resp, err := client.ListStorageContainers(nil, nil, &fltr, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.GetData() == nil {
		// No images were returned.
		return []clustermgmtv4.StorageContainer{}, nil
	}
	containers, ok := resp.GetData().([]clustermgmtv4.StorageContainer)
	if !ok {
		return nil, fmt.Errorf("failed to get data returned by ListStorageContainers(filter=%q)", fltr)
	}
	return containers, nil
}

func getCluster(
	client client,
	clusterIdentifier *v1beta1.NutanixResourceIdentifier,
) (*clustermgmtv4.Cluster, error) {
	switch {
	case clusterIdentifier.IsUUID():
		resp, err := client.GetClusterById(clusterIdentifier.UUID)
		if err != nil {
			return nil, err
		}

		cluster, ok := resp.GetData().(clustermgmtv4.Cluster)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by GetClusterById")
		}

		return &cluster, nil
	case clusterIdentifier.IsName():
		filter := fmt.Sprintf("name eq '%s'", *clusterIdentifier.Name)
		resp, err := client.ListClusters(nil, nil, &filter, nil, nil, nil)
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
