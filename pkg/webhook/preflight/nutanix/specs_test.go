// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

func TestNewConfigurationCheck(t *testing.T) {
	tests := []struct {
		name                                      string
		cluster                                   *clusterv1.Cluster
		expectedResult                            preflight.CheckResult
		expectedNutanixClusterConfigSpec          bool
		expectedWorkerNodeConfigSpecMapNotEmpty   bool
		expectedWorkerNodeConfigSpecMapEntryCount int
	}{
		{
			name: "valid cluster config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{"nutanix": {"prismCentral": {"address": "pc.example.com"}}}`),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          true,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "valid control plane config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(
										`{"controlPlane": {"nutanix": {"cluster": {"name": "cluster-from-variable"}}}}`,
									),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          true,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "valid worker config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(
														`{"nutanix": {"cluster": {"name": "cluster-from-override"}}}`,
													),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   true,
			expectedWorkerNodeConfigSpecMapEntryCount: 1,
		},
		{
			name: "invalid cluster config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{invalid-json`),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to unmarshal cluster variable \"clusterConfig\": failed to unmarshal json: invalid character 'i' looking for beginning of object key string. Review the Cluster.", ///nolint:lll // The message is long.
						Field:   "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.nutanix",
					},
				},
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "invalid worker config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(`{invalid-json`),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed:       false,
				InternalError: true,
				Causes: []preflight.Cause{
					{
						Message: "Failed to unmarshal variable \"workerConfig\": failed to unmarshal json: invalid character 'i' looking for beginning of object key string. Review the Cluster.", ///nolint:lll // The message is long.
						Field:   "$.spec.topology.workers.machineDeployments[?@.name==\"md-0\"].variables[?@.name=='workerConfig'].value.nutanix",                                                 ///nolint:lll // The field is long.
					},
				},
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "no nutanix config",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "multiple worker configs",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(
														`{"nutanix": {"cluster": {"name": "cluster-from-override"}}}`,
													),
												},
											},
										},
									},
								},
								{
									Name: "md-1",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(
														`{"nutanix": {"cluster": {"name": "cluster-from-override"}}}`,
													),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   true,
			expectedWorkerNodeConfigSpecMapEntryCount: 2,
		},
		{
			name: "worker config from cluster variables",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
							{
								Name: carenv1.WorkerConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(
										`{"nutanix": {"cluster": {"name": "cluster-from-variable"}}}`,
									),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   true,
			expectedWorkerNodeConfigSpecMapEntryCount: 1,
		},
		{
			name: "worker config with failure domain",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(
										`{"nutanix": {"failureDomains": ["fd-1", "fd-2", "fd-3"]}}`,
									),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(
														`{"nutanix": {"cluster": {"name": "worker-cluster"}, "subnets": [{"name": "worker-subnet"}]}}`,
													),
												},
											},
										},
									},
									FailureDomain: ptr.To("fd-1"),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          true,
			expectedWorkerNodeConfigSpecMapNotEmpty:   true,
			expectedWorkerNodeConfigSpecMapEntryCount: 1,
		},
		{
			name: "worker config without nutanix field",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(`{"someOtherField": true}`),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "machineDeployment without variables",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{}`),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-0",
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          false,
			expectedWorkerNodeConfigSpecMapNotEmpty:   false,
			expectedWorkerNodeConfigSpecMapEntryCount: 0,
		},
		{
			name: "mixed worker scenarios - with/without overrides and with/without failure domains",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: carenv1.ClusterConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{"nutanix": {"failureDomains": ["fd-1", "fd-2", "fd-3"]}}`),
								},
							},
							{
								Name: carenv1.WorkerConfigVariableName,
								Value: v1.JSON{
									Raw: []byte(`{"nutanix": {"cluster": {"name": "cluster-from-variable"}}}`),
								},
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "md-with-overrides",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(
														`{"nutanix": {"cluster": {"name": "cluster-from-override"}}}`,
													),
												},
											},
										},
									},
								},
								{
									Name: "md-without-overrides",
								},
								{
									Name:          "md-with-overrides-and-fd",
									FailureDomain: ptr.To("fd-1"),
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: carenv1.WorkerConfigVariableName,
												Value: v1.JSON{
													Raw: []byte(
														`{"nutanix": {"cluster": {"name": "cluster-from-override"}}}`,
													),
												},
											},
										},
									},
								},
								{
									Name:          "md-without-overrides-and-fd",
									FailureDomain: ptr.To("fd-1"),
								},
							},
						},
					},
				},
			},
			expectedResult: preflight.CheckResult{
				Allowed: true,
			},
			expectedNutanixClusterConfigSpec:          true,
			expectedWorkerNodeConfigSpecMapNotEmpty:   true,
			expectedWorkerNodeConfigSpecMapEntryCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &checkDependencies{
				cluster: tt.cluster,
				log:     testr.New(t),
			}

			check := newConfigurationCheck(cd)
			result := check.Run(context.Background())

			assert.Equal(t, tt.expectedResult, result)

			hasNutanixClusterConfigSpec := cd.nutanixClusterConfigSpec != nil
			assert.Equal(t, tt.expectedNutanixClusterConfigSpec, hasNutanixClusterConfigSpec)

			hasWorkerNodeConfigSpecMap := cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName != nil
			assert.Equal(t, tt.expectedWorkerNodeConfigSpecMapNotEmpty, hasWorkerNodeConfigSpecMap)

			if hasWorkerNodeConfigSpecMap {
				assert.Len(
					t,
					cd.nutanixWorkerNodeConfigSpecByMachineDeploymentName, tt.expectedWorkerNodeConfigSpecMapEntryCount,
				)
			}
		})
	}
}
