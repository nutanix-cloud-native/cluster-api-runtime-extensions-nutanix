// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	clustermgmtv4errors "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/error"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestInitStorageContainerChecks(t *testing.T) {
	testCases := []struct {
		name                         string
		nutanixClusterConfigSpec     *carenv1.NutanixClusterConfigSpec
		workerNodeConfigSpecByMDName map[string]*carenv1.NutanixWorkerNodeConfigSpec
		expectedChecksCount          int
		nclient                      client
	}{
		{
			name:                         "client not initialized",
			nutanixClusterConfigSpec:     nil,
			workerNodeConfigSpecByMDName: nil,
			expectedChecksCount:          0,
			nclient:                      nil,
		},
		{
			name:                         "nil cluster config",
			nutanixClusterConfigSpec:     nil,
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
			nclient:                      &mocknclient{},
		},
		{
			name: "cluster config without addons",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
			nclient:                      &mocknclient{},
		},
		{
			name: "cluster config with addons but no CSI",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{},
				},
				Addons: &carenv1.NutanixAddons{},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
			nclient:                      &mocknclient{},
		},
		{
			name: "cluster config with CSI but no control plane or worker nodes",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				Addons: &carenv1.NutanixAddons{
					CSI: &carenv1.NutanixCSI{
						Providers: carenv1.NutanixCSIProviders{
							NutanixCSI: carenv1.CSIProvider{},
						},
					},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
			nclient:                      &mocknclient{},
		},
		{
			name: "cluster config with CSI and control plane",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("my-cluster"),
							},
						},
					},
				},
				Addons: &carenv1.NutanixAddons{
					CSI: &carenv1.NutanixCSI{
						Providers: carenv1.NutanixCSIProviders{
							NutanixCSI: carenv1.CSIProvider{
								StorageClassConfigs: map[string]carenv1.StorageClassConfig{
									"test-sc": {
										Parameters: map[string]string{
											"storageContainer": "test-container",
										},
									},
								},
							},
						},
					},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          1,
			nclient:                      &mocknclient{},
		},
		{
			name: "cluster config with CSI and worker nodes",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				Addons: &carenv1.NutanixAddons{
					CSI: &carenv1.NutanixCSI{
						Providers: carenv1.NutanixCSIProviders{
							NutanixCSI: carenv1.CSIProvider{},
						},
					},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker-cluster"),
							},
						},
					},
				},
			},
			expectedChecksCount: 1,
			nclient:             &mocknclient{},
		},
		{
			name: "cluster config with CSI, control plane and worker nodes",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("cp-cluster"),
							},
						},
					},
				},
				Addons: &carenv1.NutanixAddons{
					CSI: &carenv1.NutanixCSI{
						Providers: carenv1.NutanixCSIProviders{
							NutanixCSI: carenv1.CSIProvider{},
						},
					},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker1-cluster"),
							},
						},
					},
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker2-cluster"),
							},
						},
					},
				},
			},
			expectedChecksCount: 3, // 1 for control plane, 2 for workers
			nclient:             &mocknclient{},
		},
		{
			name: "cluster config with CSI and null control plane nutanix",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: nil, // explicitly null
				},
				Addons: &carenv1.NutanixAddons{
					CSI: &carenv1.NutanixCSI{
						Providers: carenv1.NutanixCSIProviders{
							NutanixCSI: carenv1.CSIProvider{},
						},
					},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
			nclient:                      &mocknclient{},
		},
		{
			name: "cluster config with CSI and some nutanix nil workers",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				Addons: &carenv1.NutanixAddons{
					CSI: &carenv1.NutanixCSI{
						Providers: carenv1.NutanixCSIProviders{
							NutanixCSI: carenv1.CSIProvider{},
						},
					},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
				"worker-1": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker1-cluster"),
							},
						},
					},
				},
				"worker-2": {
					Nutanix: nil,
				},
			},
			expectedChecksCount: 1, // only for the defined worker-1
			nclient:             &mocknclient{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				nutanixClusterConfigSpec:                           tc.nutanixClusterConfigSpec,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tc.workerNodeConfigSpecByMDName,
				nclient: tc.nclient,
			}

			// Call the function under test
			checks := newStorageContainerChecks(cd)

			// Verify number of checks
			assert.Len(t, checks, tc.expectedChecksCount, "Wrong number of checks created")
		})
	}
}

func TestStorageContainerCheck(t *testing.T) {
	clusterName := "test-cluster"
	field := "test.field.path"

	testCases := []struct {
		name                 string
		machineSpec          *carenv1.NutanixMachineDetails
		csiSpec              *carenv1.CSIProvider
		nclient              client
		expectedResult       preflight.CheckResult
		expectedAllowed      bool
		expectedError        bool
		expectedCauseMessage string
	}{
		{
			name: "nil storage class configs",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec:              &carenv1.CSIProvider{StorageClassConfigs: nil},
			nclient:              nil,
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "Nutanix CSI Provider configuration is missing storage class configurations",
		},
		{
			name: "storage class config without parameters",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: nil,
					},
				},
			},
			nclient:         nil,
			expectedAllowed: true,
			expectedError:   false,
		},
		{
			name: "storage class config without storage container parameter",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"otherParam": "value",
						},
					},
				},
			},
			nclient:         nil,
			expectedAllowed: true,
			expectedError:   false,
		},
		{
			name: "storage container not found",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "missing-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListStorageContainersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListStorageContainersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.StorageContainer{}) // Empty list - container not found
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "Found 0 Storage Containers that match identifier \"missing-container\" on Cluster \"test-cluster\". There must be exactly 1 Storage Container that matches this identifier. Make the Storage Container identifiers unique, or use a different Storage Container, and then retry.", //nolint:lll // Message is long.
		},
		{
			name: "multiple storage containers found",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "duplicate-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListStorageContainersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListStorageContainersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.StorageContainer{
						{
							Name: ptr.To("duplicate-container"),
						},
						{
							Name: ptr.To("duplicate-container"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "Found 2 Storage Containers that match identifier \"duplicate-container\" on Cluster \"test-cluster\". There must be exactly 1 Storage Container that matches this identifier. Make the Storage Container identifiers unique, or use a different Storage Container, and then retry.", //nolint:lll // The message is long.
		},
		{
			name: "successful storage container check",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListStorageContainersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListStorageContainersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.StorageContainer{
						{
							Name: ptr.To("valid-container"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed: true,
			expectedError:   false,
		},
		{
			name: "multiple clusters found",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-456"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "Found 2 Clusters (Prism Elements) in Prism Central that match identifier \"test-cluster\". There must be exactly 1 Cluster that matches this identifier. Make the Cluster identifiers unique, and then retry.", //nolint:lll // The message is long.
		},
		{
			name: "error getting cluster",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, fmt.Errorf("API error")
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					return nil, fmt.Errorf("API error")
				},
			},
			expectedAllowed:      false,
			expectedError:        true,
			expectedCauseMessage: "Failed to check if storage container \"valid-container\" exists: failed to get cluster \"test-cluster\": API error. This is usually a temporary error. Please retry.", //nolint:lll // The message is long.
		},
		{
			name: "error listing storage containers",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					return nil, fmt.Errorf("API error listing containers")
				},
			},
			expectedAllowed:      false,
			expectedError:        true,
			expectedCauseMessage: "Failed to check if Storage Container \"valid-container\" exists in cluster \"test-cluster\": API error listing containers. This is usually a temporary error. Please retry.", //nolint:lll // The message is long.
		},
		{
			name: "error response from ListStorageContainers",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListStorageContainersApiResponse{}
					err := resp.SetData(*clustermgmtv4errors.NewErrorResponse())
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed:      false,
			expectedError:        true,
			expectedCauseMessage: "Failed to check if Storage Container \"valid-container\" exists in cluster \"test-cluster\": failed to get data returned by ListStorageContainers (filter=\"name eq 'valid-container' and clusterExtId eq 'cluster-uuid-123'\"). This is usually a temporary error. Please retry.", //nolint:lll // The message is long.
		},
		{
			name: "nil data from ListStorageContainers",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					return &clustermgmtv4.ListStorageContainersApiResponse{}, nil
				},
			},
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "Found 0 Storage Containers that match identifier \"valid-container\" on Cluster \"test-cluster\". There must be exactly 1 Storage Container that matches this identifier. Make the Storage Container identifiers unique, or use a different Storage Container, and then retry.", //nolint:lll // The message is long.
		},
		{
			name: "multiple storage class configs with success",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc-1": {
						Parameters: map[string]string{
							"otherParam": "value",
						},
					},
					"test-sc-2": {
						Parameters: map[string]string{
							"storageContainer": "valid-container",
						},
					},
					"test-sc-3": {
						Parameters: map[string]string{
							"storageContainer": "another-valid-container",
						},
					},
				},
			},
			nclient: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, nil
				},
				listClustersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To(clusterName),
							ExtId: ptr.To("cluster-uuid-123"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				listStorageContainersFunc: func(
					page,
					limit *int,
					filter,
					orderby,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListStorageContainersApiResponse,
					error,
				) {
					require.NotNil(t, filter)
					// Extract name from filter
					containerName := ""
					switch *filter {
					case "name eq 'valid-container' and clusterExtId eq 'cluster-uuid-123'":
						containerName = "valid-container"
					case "name eq 'another-valid-container' and clusterExtId eq 'cluster-uuid-123'":
						containerName = "another-valid-container"
					default:
						return nil, fmt.Errorf("filter %q does not match any storage container", *filter)
					}

					resp := &clustermgmtv4.ListStorageContainersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListStorageContainersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.StorageContainer{
						{
							Name: ptr.To(containerName),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectedAllowed: true,
			expectedError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the check function
			check := storageContainerCheck{
				machineSpec: tc.machineSpec,
				csiSpec:     tc.csiSpec,
				nclient:     tc.nclient,
				field:       field,
			}

			// Run the check
			ctx := context.Background()
			result := check.Run(ctx)

			// Verify the result
			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			assert.Equal(t, tc.expectedError, result.InternalError)

			if tc.expectedCauseMessage != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tc.expectedCauseMessage)
				assert.Equal(t, field, result.Causes[0].Field)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestGetClusters(t *testing.T) {
	testCases := []struct {
		name               string
		clusterIdentifier  *capxv1.NutanixResourceIdentifier
		client             client
		expectError        bool
		errorContains      string
		expectedClusterIDs []string
	}{
		{
			name: "get cluster by UUID - success",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid-123"),
			},
			client: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					assert.Equal(t, "test-uuid-123", *id)
					resp := &clustermgmtv4.GetClusterApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.GetClusterApiResponse"),
					}
					err := resp.SetData(
						clustermgmtv4.Cluster{
							ObjectType_: ptr.To("clustermgmt.v4.config.Cluster"),
							ExtId:       ptr.To("test-uuid-123"),
							Name:        ptr.To("test-cluster"),
						},
					)
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectError:        false,
			expectedClusterIDs: []string{"test-uuid-123"},
		},
		{
			name: "get cluster by UUID - API error",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid-error"),
			},
			client: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			expectError:   true,
			errorContains: "API error",
		},
		{
			name: "get cluster by UUID - error response",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid-invalid"),
			},
			client: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					resp := &clustermgmtv4.GetClusterApiResponse{}
					err := resp.SetData(*clustermgmtv4errors.NewErrorResponse())
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectError:   true,
			errorContains: "failed to get data returned by GetClusterById",
		},
		{
			name: "get cluster by name - success",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					assert.NotNil(t, filter)
					assert.Equal(t, "name eq 'test-cluster'", *filter)
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							ExtId: ptr.To("test-uuid-123"),
							Name:  ptr.To("test-cluster"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectError:        false,
			expectedClusterIDs: []string{"test-uuid-123"},
		},
		{
			name: "get cluster by name - API error",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-error"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					return nil, fmt.Errorf("API error")
				},
			},
			expectError:   true,
			errorContains: "API error",
		},
		{
			name: "get cluster by name - nil response",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-nil"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					return nil, nil
				},
			},
			expectError:   false,
			errorContains: "no clusters were returned",
		},
		{
			name: "get cluster by name - error response",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-nil"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{}
					err := resp.SetData(*clustermgmtv4errors.NewErrorResponse())
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectError:   true,
			errorContains: "failed to get data returned by ListClusters",
		},
		{
			name: "get cluster by name - nil data",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-nil-data"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					return &clustermgmtv4.ListClustersApiResponse{
						Data: nil,
					}, nil
				},
			},
			expectError:   false,
			errorContains: "no clusters were returned",
		},
		{
			name: "get cluster by name - no clusters found",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-not-found"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectError:   false,
			errorContains: "no clusters found with name",
		},
		{
			name: "get cluster by name - multiple clusters found",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-duplicate"),
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							ExtId: ptr.To("test-uuid-1"),
							Name:  ptr.To("test-cluster-duplicate"),
						},
						{
							ExtId: ptr.To("test-uuid-2"),
							Name:  ptr.To("test-cluster-duplicate"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			expectError:        false,
			expectedClusterIDs: []string{"test-uuid-1", "test-uuid-2"},
		},
		{
			name: "invalid identifier type",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: "invalid",
			},
			client:        &mocknclient{},
			expectError:   true,
			errorContains: "cluster identifier is missing both name and uuid",
		},
		{
			name: "nil UUID for UUID type",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: nil,
			},
			client: &mocknclient{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, fmt.Errorf("should not be called")
				},
			},
			expectError:   true,
			errorContains: "cluster identifier is missing both name and uuid",
		},
		{
			name: "nil name for Name type",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: nil,
			},
			client: &mocknclient{
				listClustersFunc: func(page,
					limit *int,
					filter,
					orderby,
					apply,
					select_ *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.ListClustersApiResponse,
					error,
				) {
					return nil, fmt.Errorf("should not be called")
				},
			},
			expectError:   true,
			errorContains: "cluster identifier is missing both name and uuid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clusters, err := getClusters(tc.client, tc.clusterIdentifier)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, clusters)
				return
			}

			require.NoError(t, err)
			assert.Len(t, tc.expectedClusterIDs, len(clusters))
			for i, cluster := range clusters {
				assert.Equal(t, tc.expectedClusterIDs[i], *cluster.ExtId)
			}
		})
	}
}
