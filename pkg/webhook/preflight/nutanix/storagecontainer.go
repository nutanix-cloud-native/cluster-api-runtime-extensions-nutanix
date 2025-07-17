// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"sync"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const (
	csiParameterKeyStorageContainer = "storageContainer"
)

type storageContainerCheck struct {
	// machineSpec is used for cluster identifier when no failure domain is configured
	machineSpec *carenv1.NutanixMachineDetails
	// failureDomainName is used for cluster identifier when failure domain is configured
	failureDomainName string
	namespace         string
	kclient           ctrlclient.Client
	// Common fields
	field   string
	csiSpec *carenv1.CSIProvider
	nclient client
}

func (c *storageContainerCheck) Name() string {
	return "NutanixStorageContainer"
}

func (c *storageContainerCheck) Run(ctx context.Context) preflight.CheckResult {
	result := preflight.CheckResult{
		Allowed: true,
	}

	if c.csiSpec.StorageClassConfigs == nil {
		result.Allowed = false
		result.Causes = append(result.Causes, preflight.Cause{
			Message: "Nutanix CSI Provider configuration is missing storage class configurations. Review the Cluster.", //nolint:lll // Message is long.
			Field:   c.field,
		})

		return result
	}

	// Get cluster identifier based on whether failure domain is configured
	var clusterIdentifier *capxv1.NutanixResourceIdentifier
	var err error

	if c.failureDomainName != "" {
		// Get cluster identifier from failure domain
		clusterIdentifier, err = c.getClusterIdentifierFromFailureDomain(ctx)
		if err != nil {
			result.Allowed = false
			if errors.IsNotFound(err) {
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"NutanixFailureDomain %q referenced in cluster was not found in the management cluster. Please create it and retry.", //nolint:lll // Message is long.
						c.failureDomainName,
					),
					Field: c.field,
				})
			} else {
				result.InternalError = true
				result.Causes = append(result.Causes, preflight.Cause{
					Message: fmt.Sprintf(
						"Failed to get cluster identifier from NutanixFailureDomain %q: %v This is usually a temporary error. Please retry.", //nolint:lll // Message is long.
						c.failureDomainName,
						err,
					),
					Field: c.field,
				})
			}
			return result
		}
	} else {
		// Use cluster identifier from machine spec
		clusterIdentifier = &c.machineSpec.Cluster
	}

	// To avoid unnecessary API calls, we delay the retrieval of clusters until we actually
	// need to check for storage containers.
	// There is only one cluster referenced in MachineDetails, so we can use sync.OnceValues
	// to ensure that we only call getClusters once, even if there are multiple storage
	// class configs.
	getClustersOnce := sync.OnceValues(func() ([]clustermgmtv4.Cluster, error) {
		return getClusters(c.nclient, clusterIdentifier)
	})

	for _, storageClassConfig := range c.csiSpec.StorageClassConfigs {
		if storageClassConfig.Parameters == nil {
			continue
		}

		storageContainerName, ok := storageClassConfig.Parameters[csiParameterKeyStorageContainer]
		if !ok {
			continue
		}

		clusters, err := getClustersOnce()
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to check if storage container %q exists: failed to get cluster %q: %s. This is usually a temporary error. Please retry.", ///nolint:lll // Message is long.
					storageContainerName,
					clusterIdentifier,
					err,
				),
				Field: c.field,
			})
			continue
		}

		if len(clusters) != 1 {
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found %d Clusters (Prism Elements) in Prism Central that match identifier %q. There must be exactly 1 Cluster that matches this identifier. Use a unique Cluster name, or identify the Cluster by its UUID, then retry.", ///nolint:lll // Message is long.
					len(clusters),
					clusterIdentifier,
				),
				Field: c.field,
			})
			continue
		}

		// Found exactly one cluster.
		cluster := &clusters[0]

		containers, err := getStorageContainers(c.nclient, *cluster.ExtId, storageContainerName)
		if err != nil {
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Failed to check if Storage Container %q exists in cluster %q: %s. This is usually a temporary error. Please retry.", ///nolint:lll // Message is long.
					storageContainerName,
					clusterIdentifier,
					err,
				),
				Field: c.field,
			})
			continue
		}

		// Because Storage Container names are unique within a Cluster, we will either find exactly one
		// Storage Container with the given name, or none at all.
		switch len(containers) {
		case 0:
			result.Allowed = false
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found no Storage Containers with name %q on Cluster %q. Create a Storage Container with this name on Cluster %q, and then retry.", ///nolint:lll // Message is long.
					storageContainerName,
					clusterIdentifier,
					clusterIdentifier,
				),
				Field: c.field,
			})
		case 1:
			continue
		default: // 2 or more Storage Containers with the same name found on the same Cluster.
			// This is an unexpected situation, as Storage Container names should be unique within a Cluster.
			// We log this as an internal error, as it indicates a potential issue with the Nutanix API or the
			// underlying data.
			result.Allowed = false
			result.InternalError = true
			result.Causes = append(result.Causes, preflight.Cause{
				Message: fmt.Sprintf(
					"Found %d Storage Containers with name %q on Cluster %q. This should not happen under normal circumstances. Please report.", ///nolint:lll // Message is long.
					len(containers),
					storageContainerName,
					clusterIdentifier,
				),
				Field: c.field,
			})
		}
	}

	return result
}

// getClusterIdentifierFromFailureDomain fetches the failure domain and returns the cluster identifier.
func (c *storageContainerCheck) getClusterIdentifierFromFailureDomain(
	ctx context.Context,
) (*capxv1.NutanixResourceIdentifier, error) {
	fdObj := &capxv1.NutanixFailureDomain{}
	fdKey := ctrlclient.ObjectKey{Name: c.failureDomainName, Namespace: c.namespace}
	if err := c.kclient.Get(ctx, fdKey, fdObj); err != nil {
		return nil, err
	}

	return &fdObj.Spec.PrismElementCluster, nil
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

	if cd.nutanixClusterConfigSpec != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane != nil &&
		cd.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {

		controlPlaneNutanix := cd.nutanixClusterConfigSpec.ControlPlane.Nutanix

		// Check if failureDomains are configured for control plane
		if len(controlPlaneNutanix.FailureDomains) > 0 && cd.cluster != nil && cd.kclient != nil {
			// Create a check for each failure domain
			for _, fdName := range controlPlaneNutanix.FailureDomains {
				if fdName != "" {
					checks = append(checks,
						&storageContainerCheck{
							failureDomainName: fdName,
							namespace:         cd.cluster.Namespace,
							kclient:           cd.kclient,
							field:             "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.failureDomains",
							csiSpec:           &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
							nclient:           cd.nclient,
						},
					)
				}
			}
		} else {
			checks = append(checks,
				&storageContainerCheck{
					machineSpec: &controlPlaneNutanix.MachineDetails,
					field:       "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails",
					csiSpec:     &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
					nclient:     cd.nclient,
				},
			)
		}
	}

	for mdName, nutanixWorkerNodeConfigSpec := range cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName {
		if nutanixWorkerNodeConfigSpec.Nutanix != nil {
			// Check if failureDomain is configured for this machine deployment
			if fdName, ok := cd.failureDomainByMachineDeploymentName[mdName]; ok && fdName != "" && cd.cluster != nil && cd.kclient != nil {
				// Use failure domain for cluster information
				checks = append(checks,
					&storageContainerCheck{
						failureDomainName: fdName,
						namespace:         cd.cluster.Namespace,
						kclient:           cd.kclient,

						field: fmt.Sprintf(
							"$.spec.topology.workers.machineDeployments[?@.name==%q].failureDomain",
							mdName,
						),
						csiSpec: &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
						nclient: cd.nclient,
					},
				)
			} else {
				// Use machine details for cluster information
				checks = append(checks,
					&storageContainerCheck{
						machineSpec: &nutanixWorkerNodeConfigSpec.Nutanix.MachineDetails,
						//nolint:lll // The field is long.
						field: fmt.Sprintf(
							"$.spec.topology.workers.machineDeployments[?@.name==%q].variables[?@.name=workerConfig].value.nutanix.machineDetails",
							mdName,
						),
						csiSpec: &cd.nutanixClusterConfigSpec.Addons.CSI.Providers.NutanixCSI,
						nclient: cd.nclient,
					},
				)
			}
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
		return nil, fmt.Errorf("failed to get data returned by ListStorageContainers (filter=%q)", fltr)
	}
	return containers, nil
}

func getClusters(
	client client,
	clusterIdentifier *capxv1.NutanixResourceIdentifier,
) ([]clustermgmtv4.Cluster, error) {
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
		return []clustermgmtv4.Cluster{cluster}, nil
	case clusterIdentifier.IsName():
		filter := fmt.Sprintf("name eq '%s'", *clusterIdentifier.Name)
		resp, err := client.ListClusters(nil, nil, &filter, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.GetData() == nil {
			// No clusters were returned.
			return []clustermgmtv4.Cluster{}, nil
		}

		clusters, ok := resp.GetData().([]clustermgmtv4.Cluster)
		if !ok {
			return nil, fmt.Errorf("failed to get data returned by ListClusters")
		}
		return clusters, nil
	default:
		return nil, fmt.Errorf("cluster identifier is missing both name and uuid")
	}
}
