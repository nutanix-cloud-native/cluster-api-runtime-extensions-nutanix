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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestInitStorageContainerChecks(t *testing.T) {
	testScheme := runtime.NewScheme()
	require.NoError(t, capxv1.AddToScheme(testScheme))

	fd1 := &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{Name: "fd-1", Namespace: "default"},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("pe-cluster-from-fd-1"),
			},
		},
	}
	fd2 := &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{Name: "fd-2", Namespace: "default"},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("pe-cluster-from-fd-2"),
			},
		},
	}
	fd3 := &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{Name: "fd-3", Namespace: "default"},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("pe-cluster-from-fd-3"),
			},
		},
	}
	fd4 := &capxv1.NutanixFailureDomain{
		ObjectMeta: metav1.ObjectMeta{Name: "fd-4", Namespace: "default"},
		Spec: capxv1.NutanixFailureDomainSpec{
			PrismElementCluster: capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("pe-cluster-from-fd-4"),
			},
		},
	}

	fakeKubeClient := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(fd1, fd2, fd3, fd4).Build()

	clusterObj := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	testCases := []struct {
		name                                 string
		nutanixClusterConfigSpec             *carenv1.NutanixClusterConfigSpec
		workerNodeConfigSpecByMDName         map[string]*carenv1.NutanixWorkerNodeConfigSpec
		failureDomainByMachineDeploymentName map[string]string
		expectedChecksCount                  int
		nclient                              client
		kclient                              ctrlclient.Client
		cluster                              *clusterv1.Cluster
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
			nclient:                      &clientWrapper{},
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
			nclient:                      &clientWrapper{},
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
			nclient:                      &clientWrapper{},
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
			nclient:                      &clientWrapper{},
		},
		{
			name: "cluster config with CSI and control plane without failure domains",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient:                      &clientWrapper{},
			kclient:                      fakeKubeClient,
			cluster:                      clusterObj,
		},
		{
			name: "cluster config with CSI and control plane with failure domains",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"fd-1", "fd-2", "fd-3"},
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
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
			expectedChecksCount:          3,
			nclient:                      &clientWrapper{},
			kclient:                      fakeKubeClient,
			cluster:                      clusterObj,
		},
		{
			name: "cluster config with CSI and worker nodes without failureDomain",
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
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker-cluster"),
							},
						},
					},
				},
			},
			expectedChecksCount: 1,
			nclient:             &clientWrapper{},
			kclient:             fakeKubeClient,
			cluster:             clusterObj,
		},
		{
			name: "cluster config with CSI and worker nodes with failureDomain",
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
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker-cluster"),
							},
						},
					},
				},
			},
			failureDomainByMachineDeploymentName: map[string]string{"worker-1": "fd-4"},
			expectedChecksCount:                  1,
			nclient:                              &clientWrapper{},
			kclient:                              fakeKubeClient,
			cluster:                              clusterObj,
		},
		{
			name: "cluster config with CSI, control plane and worker nodes",
			nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
				ControlPlane: &carenv1.NutanixControlPlaneSpec{
					Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
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
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker1-cluster"),
							},
						},
					},
				},
				"worker-2": {
					Nutanix: &carenv1.NutanixWorkerNodeSpec{
						MachineDetails: carenv1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("worker2-cluster"),
							},
						},
					},
				},
			},
			expectedChecksCount: 3, // 1 for control plane, 2 for workers
			nclient:             &clientWrapper{},
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
			nclient:                      &clientWrapper{},
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
							Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient:             &clientWrapper{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cd := &checkDependencies{
				nutanixClusterConfigSpec:                           tc.nutanixClusterConfigSpec,
				nutanixWorkerNodeConfigSpecByMachineDeploymentName: tc.workerNodeConfigSpecByMDName,
				failureDomainByMachineDeploymentName:               tc.failureDomainByMachineDeploymentName,
				nclient:                                            tc.nclient,
				kclient:                                            tc.kclient,
				cluster:                                            tc.cluster,
				pcVersion:                                          "7.3.0",
			}
			if cd.failureDomainByMachineDeploymentName == nil {
				cd.failureDomainByMachineDeploymentName = map[string]string{}
			}

			// Call the function under test
			checks := newStorageContainerChecks(cd)

			// Verify number of checks
			assert.Len(t, checks, tc.expectedChecksCount, "Wrong number of checks created")
		})
	}
}

func TestStorageContainerCheck(t *testing.T) {
	testScheme := runtime.NewScheme()
	require.NoError(t, capxv1.AddToScheme(testScheme))

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
		expectedField        string
		failureDomainName    string
		kclient              ctrlclient.Client
		namespace            string
	}{
		{
			name: "nil storage class configs",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: &capxv1.NutanixResourceIdentifier{
					Type: capxv1.NutanixIdentifierName,
					Name: ptr.To(clusterName),
				},
			},
			csiSpec:              &carenv1.CSIProvider{StorageClassConfigs: nil},
			nclient:              nil,
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "Nutanix CSI Provider configuration is missing storage class configurations. Review the Cluster.", //nolint:lll // The message is long.
		},
		{
			name: "storage class config without parameters",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: &capxv1.NutanixResourceIdentifier{
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
				Cluster: &capxv1.NutanixResourceIdentifier{
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
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
			expectedCauseMessage: "Found no Storage Containers with name \"missing-container\" on Cluster \"test-cluster\". Create a Storage Container with this name on Cluster \"test-cluster\", and then retry.", //nolint:lll // Message is long.
		},
		{
			name: "multiple storage containers with same name in same cluster found",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil,
						nil
				},

				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
			expectedError:        true,
			expectedCauseMessage: "Found 2 Storage Containers with name \"duplicate-container\" on Cluster \"test-cluster\". This should not happen under normal circumstances. Please report.", //nolint:lll // The message is long.
		},
		{
			name: "successful storage container check",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				ListClustersFunc: func(
					ctx context.Context,
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
			expectedCauseMessage: "Found 2 Clusters (Prism Elements) in Prism Central that match identifier \"test-cluster\". There must be exactly 1 Cluster that matches this identifier. Use a unique Cluster name, or identify the Cluster by its UUID, then retry.", //nolint:lll // The message is long.
		},
		{
			name: "error getting cluster",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, fmt.Errorf("API error")
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
			expectedCauseMessage: "Found no Storage Containers with name \"valid-container\" on Cluster \"test-cluster\". Create a Storage Container with this name on Cluster \"test-cluster\", and then retry.", //nolint:lll // The message is long.
		},
		{
			name: "multiple storage class configs with success",
			machineSpec: &carenv1.NutanixMachineDetails{
				Cluster: &capxv1.NutanixResourceIdentifier{
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
			nclient: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					return nil, nil
				},
				ListClustersFunc: func(
					ctx context.Context,
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
				ListStorageContainersFunc: func(
					ctx context.Context,
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
		{
			name: "successful storage container check with failure domain",
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "fd-container",
						},
					},
				},
			},
			nclient: &clientWrapper{
				ListClustersFunc: func(
					ctx context.Context,
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
					require.NotNil(t, filter)
					assert.Equal(
						t,
						"name eq 'pe-cluster-from-fd'",
						*filter,
						"filter should be based on failure domain",
					)

					resp := &clustermgmtv4.ListClustersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListClustersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.Cluster{
						{
							Name:  ptr.To("pe-cluster-from-fd"),
							ExtId: ptr.To("cluster-uuid-fd"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
				ListStorageContainersFunc: func(
					ctx context.Context,
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
					assert.Equal(
						t,
						"name eq 'fd-container' and clusterExtId eq 'cluster-uuid-fd'",
						*filter,
						"filter should be based on storage container and cluster from failure domain",
					)

					resp := &clustermgmtv4.ListStorageContainersApiResponse{
						ObjectType_: ptr.To("clustermgmt.v4.config.ListStorageContainersApiResponse"),
					}
					err := resp.SetData([]clustermgmtv4.StorageContainer{
						{
							Name: ptr.To("fd-container"),
						},
					})
					require.NoError(t, err)
					return resp, nil
				},
			},
			failureDomainName: "my-fd",
			kclient: fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(&capxv1.NutanixFailureDomain{
					ObjectMeta: metav1.ObjectMeta{Name: "my-fd", Namespace: "test-ns"},
					Spec: capxv1.NutanixFailureDomainSpec{
						PrismElementCluster: capxv1.NutanixResourceIdentifier{
							Type: capxv1.NutanixIdentifierName,
							Name: ptr.To("pe-cluster-from-fd"),
						},
					},
				}).Build(),
			namespace:       "test-ns",
			expectedAllowed: true,
			expectedError:   false,
		},
		{
			name: "failure domain not found",
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "some-container",
						},
					},
				},
			},
			nclient:              &clientWrapper{},
			failureDomainName:    "non-existent-fd",
			kclient:              fake.NewClientBuilder().WithScheme(testScheme).Build(), // Empty client
			namespace:            "test-ns",
			expectedAllowed:      false,
			expectedError:        false,
			expectedCauseMessage: "NutanixFailureDomain \"non-existent-fd\" was not found in the management cluster. Please create it and retry.", //nolint:lll // The message is long.
			expectedField:        field + ".failureDomain",
		},
		{
			name: "error getting failure domain",
			csiSpec: &carenv1.CSIProvider{
				StorageClassConfigs: map[string]carenv1.StorageClassConfig{
					"test-sc": {
						Parameters: map[string]string{
							"storageContainer": "some-container",
						},
					},
				},
			},
			nclient:           &clientWrapper{},
			failureDomainName: "fd-with-error",
			kclient: &mockKubeClient{
				SubResourceClient: nil,
				getFunc: func(
					ctx context.Context,
					key ctrlclient.ObjectKey,
					obj ctrlclient.Object,
					opts ...ctrlclient.GetOption,
				) error {
					return fmt.Errorf("kube API error")
				},
			},
			namespace:            "test-ns",
			expectedAllowed:      false,
			expectedError:        true,
			expectedCauseMessage: "Failed to get NutanixFailureDomain \"fd-with-error\": kube API error. This is usually a temporary error. Please retry.", //nolint:lll // The message is long.
			expectedField:        field + ".failureDomain",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create the check function
			check := storageContainerCheck{
				machineSpec:       tc.machineSpec,
				csiSpec:           tc.csiSpec,
				nclient:           tc.nclient,
				field:             field,
				failureDomainName: tc.failureDomainName,
				kclient:           tc.kclient,
				namespace:         tc.namespace,
			}

			// Run the check
			ctx := context.Background()
			result := check.Run(ctx)

			// Verify the result
			assert.Equal(t, tc.expectedAllowed, result.Allowed)
			assert.Equal(t, tc.expectedError, result.InternalError)

			if !tc.expectedAllowed {
				require.NotEmpty(t, result.Causes)

				if tc.expectedCauseMessage != "" {
					assert.Equal(t, tc.expectedCauseMessage, result.Causes[0].Message)
				}

				if tc.expectedField != "" {
					assert.Equal(t, tc.expectedField, result.Causes[0].Field)
				}
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
			client: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
					assert.Equal(t, "test-uuid-123", *uuid)
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
			client: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
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
			client: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			client:        &clientWrapper{},
			expectError:   true,
			errorContains: "cluster identifier is missing both name and uuid",
		},
		{
			name: "nil UUID for UUID type",
			clusterIdentifier: &capxv1.NutanixResourceIdentifier{
				Type: capxv1.NutanixIdentifierUUID,
				UUID: nil,
			},
			client: &clientWrapper{
				GetClusterByIdFunc: func(
					ctx context.Context,
					uuid *string,
					args ...map[string]interface{},
				) (
					*clustermgmtv4.GetClusterApiResponse,
					error,
				) {
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
			client: &clientWrapper{
				ListClustersFunc: func(ctx context.Context, page,
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
			clusters, err := getClusters(context.Background(), tc.client, tc.clusterIdentifier)

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
