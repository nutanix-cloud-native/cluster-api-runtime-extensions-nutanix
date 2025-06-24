// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	clustermgmtv4errors "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/error"
	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	csiParameterKeyStorageContainer = "storageContainer"
)

type storageContainerCheck struct {
	nodeSpec *carenv1.NutanixNodeSpec
	field    string
	csiSpec  *carenv1.CSIProvider
	nclient  client
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
		result.Error = true
		result.Causes = append(result.Causes, preflight.Cause{
			Message: fmt.Sprintf(
				"no storage container found for cluster %q",
				*c.nodeSpec.MachineDetails.Cluster.Name,
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
				*c.nodeSpec.MachineDetails.Cluster.Name,
			),
			Field: c.field,
		})

		return result
	}

	for _, storageClassConfig := range c.csiSpec.StorageClassConfigs {
		if storageClassConfig.Parameters == nil {
			continue
		}

		storageContainer, ok := storageClassConfig.Parameters[csiParameterKeyStorageContainer]
		if !ok {
			continue
		}

		if _, err := getStorageContainer(c.nclient, c.nodeSpec, storageContainer); err != nil {
			result.Allowed = false
			result.Error = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"failed to check if storage container named %q exists: %s",
					storageContainer,
					err,
				),
				Field: c.field,
			})

			return result
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
				nodeSpec: cd.nutanixClusterConfigSpec.ControlPlane.Nutanix,
				field:    "cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix",
				csiSpec:  &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
				nclient:  cd.nclient,
			},
		)
	}

	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			checks = append(checks,
				&storageContainerCheck{
					nodeSpec: nutanixWorkerNodeConfigSpec.Nutanix,
					field: fmt.Sprintf(
						"cluster.spec.topology.workers.machineDeployments[.name=%s]"+
							".variables[.name=workerConfig].value.nutanix",
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

func getStorageContainer(
	client client,
	nodeSpec *carenv1.NutanixNodeSpec,
	storageContainerName string,
) (*clustermgmtv4.StorageContainer, error) {
	cluster, err := getCluster(client, &nodeSpec.MachineDetails.Cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	fltr := fmt.Sprintf("name eq '%s' and clusterExtId eq '%s'", storageContainerName, *cluster.ExtId)
	resp, err := client.ListStorageContainers(nil, nil, &fltr, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list storage containers: %w", err)
	}

	switch resp.GetData().(type) {
	case nil:
		return nil, fmt.Errorf("failed to find a matching storage container")

	case clustermgmtv4errors.ErrorResponse:
		errResp, ok := resp.GetData().(clustermgmtv4errors.ErrorResponse)
		if !ok {
			return nil, fmt.Errorf("failed to parse error response from %v", resp.GetData())
		}

		return nil, fmt.Errorf("failed to list storage containers: %v", errResp.GetError())

	case []clustermgmtv4.StorageContainer:
		containers, ok := resp.GetData().([]clustermgmtv4.StorageContainer)
		if !ok {
			return nil, fmt.Errorf("failed to parse storage containers from %v", resp.GetData())
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

	return nil, fmt.Errorf("unexpected response type from ListStorageContainers(filter=%q): %T", fltr, resp.GetData())
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
