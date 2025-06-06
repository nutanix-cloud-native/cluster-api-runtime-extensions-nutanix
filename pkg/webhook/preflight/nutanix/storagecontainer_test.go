// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr/testr"
	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
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
	}{
		{
			name:                         "nil cluster config",
			nutanixClusterConfigSpec:     nil,
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
		},
		{
			name: "cluster config without addons",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
		},
		{
			name: "cluster config with addons but no CSI",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{},
				},
				Addons: &carenv1.NutanixAddons{},
			},
			workerNodeConfigSpecByMDName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{},
			expectedChecksCount:          0,
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
		},
		{
			name: "cluster config with CSI and control plane",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{
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
					Nutanix: &carenv1.NutanixNodeSpec{
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
		},
		{
			name: "cluster config with CSI, control plane and worker nodes",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixNodeSpec{
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
					Nutanix: &carenv1.NutanixNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker1-cluster"),
							},
						},
					},
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixNodeSpec{
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
					Nutanix: &carenv1.NutanixNodeSpec{
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := testr.New(t)

			// Create checker
			checker := &nutanixChecker{
				log:                      logger,
				nutanixClusterConfigSpec: tc.nutanixClusterConfigSpec,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tc.workerNodeConfigSpecByMDName,
				initStorageContainerChecksFunc:                     initStorageContainerChecks,
			}

			// Set up a tracing function to replace storageContainerCheck
			var capturedFields []string
			checker.storageContainerCheckFunc = func(
				n *nutanixChecker,
				nodeSpec *carenv1.NutanixNodeSpec,
				field string,
				csiSpec *carenv1.CSIProvider,
			) preflight.Check {
				capturedFields = append(capturedFields, field)
				return func(ctx context.Context) preflight.CheckResult {
					return preflight.CheckResult{Name: "StorageContainerCheck:" + field}
				}
			}

			// Call the function under test
			checks := checker.initStorageContainerChecksFunc(checker)

			// Verify number of checks
			assert.Len(t, checks, tc.expectedChecksCount, "Wrong number of checks created")

			// Verify number of captured fields matches number of checks
			assert.Len(t, capturedFields, tc.expectedChecksCount, "Wrong number of fields captured")

			// Additional verification that each field is correctly formatted
			for _, field := range capturedFields {
				if tc.nutanixClusterConfigSpec != nil && tc.nutanixClusterConfigSpec.ControlPlane != nil &&
					tc.nutanixClusterConfigSpec.ControlPlane.Nutanix != nil {
					expectedCPField := "cluster.spec.topology[.name=clusterConfig].value.controlPlane.nutanix"
					if field == expectedCPField {
						continue
					}
				}
				// If not CP field, should be worker field with MD name
				assert.Contains(t, field, "cluster.spec.topology.workers.machineDeployments[.name=")
				assert.Contains(t, field, "].variables[.name=workerConfig].value.nutanix")
			}
		})
	}
}

func TestStorageContainerCheck(t *testing.T) {
	clusterName := "test-cluster"
	fieldPath := "test.field.path"

	testCases := []struct {
		name                 string
		nodeSpec             *carenv1.NutanixNodeSpec
		csiSpec              *carenv1.CSIProvider
		mockv4client         *mockv4client
		expectedResult       preflight.CheckResult
		expectedAllowed      bool
		expectedError        bool
		expectedCauseMessage string
	}{
		{
			name: "nil CSI spec",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
				},
			},
			csiSpec:              nil,
			mockv4client:         nil,
			expectedAllowed:      false,
			expectedError:        true,
			expectedCauseMessage: fmt.Sprintf("no storage container found for cluster %q", clusterName),
		},
		{
			name: "nil storage class configs",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
				},
			},
			csiSpec:              &carenv1.CSIProvider{StorageClassConfigs: nil},
			mockv4client:         nil,
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: fmt.Sprintf("no storage class configs found for cluster %q", clusterName),
		},
		{
			name: "storage class config without parameters",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
				},
			},
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: nil,
					},
				},
			},
			mockv4client:    nil,
			expectedAllowed: true,
			expectedError:   false,
		},
		{
			name: "storage class config without storage container parameter",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client:    nil,
			expectedAllowed: true,
			expectedError:   false,
		},
		{
			name: "storage container not found",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
			expectedAllowed: false,
			expectedError:   true,
			expectedCauseMessage: "failed to check if storage container named \"missing-container\" exists:" +
				" no storage container named \"missing-container\" found on cluster named",
		},
		{
			name: "multiple storage containers found",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
			expectedAllowed: false,
			expectedError:   true,
			expectedCauseMessage: "failed to check if storage container named \"duplicate-container\" exists:" +
				" multiple storage containers found with name",
		},
		{
			name: "successful storage container check",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
			name: "error getting cluster",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
			expectedAllowed: false,
			expectedError:   true,
			expectedCauseMessage: "failed to check if storage container named \"valid-container\" exists:" +
				" failed to get cluster: API error",
		},
		{
			name: "error listing storage containers",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
			expectedAllowed: false,
			expectedError:   true,
			expectedCauseMessage: "failed to check if storage container named \"valid-container\" exists:" +
				" failed to list storage containers: API error listing containers",
		},
		{
			name: "invalid response data type",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
					// Return a non-nil response but with nil Data or wrong type to simulate data conversion error
					return &clustermgmtv4.ListStorageContainersApiResponse{
						ObjectType_: ptr.To("wrong-data-type"),
					}, nil
				},
			},
			expectedAllowed: false,
			expectedError:   true,
			expectedCauseMessage: "failed to check if storage container named \"valid-container\" exists:" +
				" failed to get data returned by ListStorageContainers",
		},
		{
			name: "multiple storage class configs with success",
			nodeSpec: &carenv1.NutanixNodeSpec{
				MachineDetails: carenv1.NutanixMachineDetails{
					Cluster: capxv1.NutanixResourceIdentifier{
						Type: capxv1.NutanixIdentifierName,
						Name: ptr.To(clusterName),
					},
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
			mockv4client: &mockv4client{
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
			logger := testr.New(t)

			// Create checker with mock v4 client
			checker := &nutanixChecker{
				log:      logger,
				v4client: tc.mockv4client,
			}

			// Create the check function
			checkFunc := storageContainerCheck(checker, tc.nodeSpec, fieldPath, tc.csiSpec)

			// Run the check
			ctx := context.Background()
			result := checkFunc(ctx)

			// Verify the result
			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			assert.Equal(t, tc.expectedError, result.Error)

			if tc.expectedCauseMessage != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tc.expectedCauseMessage)
				assert.Equal(t, fieldPath, result.Causes[0].Field)
			} else {
				assert.Empty(t, result.Causes)
			}
		})
	}
}

func TestGetCluster(t *testing.T) {
	testCases := []struct {
		name              string
		clusterIdentifier *capxv1.NutanixResourceIdentifier
		mockv4client      *mockv4client
		expectError       bool
		errorContains     string
		expectedClusterID string
	}{
		{
			name: "get cluster by UUID - success",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid-123"),
			},
			mockv4client: &mockv4client{
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
			expectError:       false,
			expectedClusterID: "test-uuid-123",
		},
		{
			name: "get cluster by UUID - API error",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid-error"),
			},
			mockv4client: &mockv4client{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					return nil, fmt.Errorf("API error")
				},
			},
			expectError:   true,
			errorContains: "API error",
		},
		{
			name: "get cluster by UUID - invalid response data",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: ptr.To("test-uuid-invalid"),
			},
			mockv4client: &mockv4client{
				getClusterByIdFunc: func(id *string) (*clustermgmtv4.GetClusterApiResponse, error) {
					// Return an invalid data type
					resp := &clustermgmtv4.GetClusterApiResponse{
						ObjectType_: ptr.To("wrong-data-type"),
					}
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
			mockv4client: &mockv4client{
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
			expectError:       false,
			expectedClusterID: "test-uuid-123",
		},
		{
			name: "get cluster by name - API error",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-error"),
			},
			mockv4client: &mockv4client{
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
			mockv4client: &mockv4client{
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
			expectError:   true,
			errorContains: "no clusters were returned",
		},
		{
			name: "get cluster by name - nil data",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-nil-data"),
			},
			mockv4client: &mockv4client{
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
			expectError:   true,
			errorContains: "no clusters were returned",
		},
		{
			name: "get cluster by name - no clusters found",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-not-found"),
			},
			mockv4client: &mockv4client{
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
			expectError:   true,
			errorContains: "no clusters found with name",
		},
		{
			name: "get cluster by name - multiple clusters found",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("test-cluster-duplicate"),
			},
			mockv4client: &mockv4client{
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
			expectError:   true,
			errorContains: "multiple clusters found with name",
		},
		{
			name: "invalid identifier type",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: "invalid",
			},
			mockv4client:  &mockv4client{},
			expectError:   true,
			errorContains: "cluster identifier is missing both name and uuid",
		},
		{
			name: "nil UUID for UUID type",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: nil,
			},
			mockv4client: &mockv4client{
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
			mockv4client: &mockv4client{
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
			cluster, err := getCluster(tc.mockv4client, tc.clusterIdentifier)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorContains)
				assert.Nil(t, cluster)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cluster)
				assert.Equal(t, tc.expectedClusterID, *cluster.ExtId)
			}
		})
	}
}
