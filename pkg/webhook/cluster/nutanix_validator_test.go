// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func mustMarshalJSON(obj any) apiextensionsv1.JSON {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return apiextensionsv1.JSON{Raw: data}
}

func TestValidatePrismCentralIPNotInLoadBalancerIPRange(t *testing.T) {
	tests := []struct {
		name                             string
		pcEndpoint                       v1alpha1.NutanixPrismCentralEndpointSpec
		serviceLoadBalancerConfiguration *v1alpha1.ServiceLoadBalancer
		expectedErr                      error
	}{
		{
			name: "PC IP not in range",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			serviceLoadBalancerConfiguration: &v1alpha1.ServiceLoadBalancer{
				Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
				Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
					AddressRanges: []v1alpha1.AddressRange{
						{Start: "192.168.1.10", End: "192.168.1.20"},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "PC IP in range",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.15:9440",
			},
			serviceLoadBalancerConfiguration: &v1alpha1.ServiceLoadBalancer{
				Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
				Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
					AddressRanges: []v1alpha1.AddressRange{
						{Start: "192.168.1.10", End: "192.168.1.20"},
					},
				},
			},
			expectedErr: fmt.Errorf(
				"Prism Central IP %q must not be part of MetalLB address range %q-%q",
				"192.168.1.15",
				"192.168.1.10",
				"192.168.1.20",
			),
		},
		{
			name: "Invalid Prism Central URL",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "invalid-url",
			},
			serviceLoadBalancerConfiguration: &v1alpha1.ServiceLoadBalancer{
				Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
				Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
					AddressRanges: []v1alpha1.AddressRange{
						{Start: "192.168.1.10", End: "192.168.1.20"},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "Service Load Balancer Configuration is nil",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			serviceLoadBalancerConfiguration: nil,
			expectedErr:                      nil,
		},
		{
			name: "Provider is not MetalLB",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			serviceLoadBalancerConfiguration: &v1alpha1.ServiceLoadBalancer{
				Provider: "other-provider",
				Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
					AddressRanges: []v1alpha1.AddressRange{
						{Start: "192.168.1.10", End: "192.168.1.20"},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrismCentralIPNotInLoadBalancerIPRange(
				tt.pcEndpoint,
				tt.serviceLoadBalancerConfiguration,
			)

			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePrismCentralIPDoesNotEqualControlPlaneIP(t *testing.T) {
	tests := []struct {
		name                     string
		pcEndpoint               v1alpha1.NutanixPrismCentralEndpointSpec
		controlPlaneEndpointSpec v1alpha1.ControlPlaneEndpointSpec
		expectedErr              error
	}{
		{
			name: "Different IPs",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.2",
			},
			expectedErr: nil,
		},
		{
			name: "Same IPs",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.1",
			},
			expectedErr: fmt.Errorf(
				"Prism Central and control plane endpoint cannot have the same IP %q",
				"192.168.1.1",
			),
		},
		{
			name: "Control Plane IP specified as hostname",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "dummy-hostname",
			},
			expectedErr: nil,
		},
		{
			name: "Invalid Prism Central URL",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "invalid-url",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.2",
			},
			expectedErr: nil,
		},
		{
			name: "Prism Central URL is FQDN",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://example.com:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.2",
			},
			expectedErr: nil,
		},
		{
			name: "With KubeVIP ovveride and same PC and Control Plane IPs",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.2",
				VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
					Provider: "KubeVIP",
					Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
						Address: "192.168.1.1",
					},
				},
			},
			expectedErr: fmt.Errorf(
				"Prism Central and control plane endpoint cannot have the same IP %q",
				"192.168.1.1",
			),
		},
		{
			name: "With KubeVIP override and different PC and Control Plane IPs",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.2:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.2",
				VirtualIPSpec: &v1alpha1.ControlPlaneVirtualIPSpec{
					Provider: "KubeVIP",
					Configuration: &v1alpha1.ControlPlaneVirtualIPConfiguration{
						Address: "192.168.1.1",
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrismCentralIPDoesNotEqualControlPlaneIP(
				tt.pcEndpoint,
				tt.controlPlaneEndpointSpec,
			)
			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTopologyFailureDomainConfig(t *testing.T) {
	tests := []struct {
		name          string
		clusterConfig *variables.ClusterConfigSpec
		cluster       *clusterv1.Cluster
		expectedErr   string
	}{
		{
			name: "Control plane: failureDomains configured, machineDetails has cluster - should error",
			clusterConfig: &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"fd1"},
						MachineDetails: v1alpha1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("test-cluster"),
							},
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			expectedErr: `"cluster" must not be set when failureDomains are configured`,
		},
		{
			name: "Control plane: failureDomains configured, machineDetails has subnets - should error",
			clusterConfig: &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"fd1"},
						MachineDetails: v1alpha1.NutanixMachineDetails{
							Subnets: []capxv1.NutanixResourceIdentifier{
								{
									Type: capxv1.NutanixIdentifierName,
									Name: ptr.To("test-subnet"),
								},
							},
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			expectedErr: `"subnets" must not be set when failureDomains are configured`,
		},
		{
			name: "Control plane: failureDomains configured, machineDetails clean - should pass",
			clusterConfig: &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{"fd1"},
						MachineDetails: v1alpha1.NutanixMachineDetails{
							VCPUsPerSocket: 2,
							VCPUSockets:    2,
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			expectedErr: "",
		},
		{
			name: "Control plane: no failureDomains, machineDetails missing cluster - should error",
			clusterConfig: &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{},
						MachineDetails: v1alpha1.NutanixMachineDetails{
							Subnets: []capxv1.NutanixResourceIdentifier{
								{
									Type: capxv1.NutanixIdentifierName,
									Name: ptr.To("test-subnet"),
								},
							},
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			expectedErr: `"cluster" must be set when failureDomains are not configured`,
		},
		{
			name: "Control plane: no failureDomains, machineDetails missing subnets - should error",
			clusterConfig: &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{},
						MachineDetails: v1alpha1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("test-cluster"),
							},
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			expectedErr: `"subnets" must be set when failureDomains are not configured`,
		},
		{
			name: "Control plane: no failureDomains, machineDetails complete - should pass",
			clusterConfig: &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						FailureDomains: []string{},
						MachineDetails: v1alpha1.NutanixMachineDetails{
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: ptr.To("test-cluster"),
							},
							Subnets: []capxv1.NutanixResourceIdentifier{
								{
									Type: capxv1.NutanixIdentifierName,
									Name: ptr.To("test-subnet"),
								},
							},
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
				},
			},
			expectedErr: "",
		},
		{
			name:          "Worker: failureDomain configured, variable overrides have cluster - should error",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name:          "worker-1",
									FailureDomain: ptr.To("fd1"),
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															Cluster: &capxv1.NutanixResourceIdentifier{
																Type: capxv1.NutanixIdentifierName,
																Name: ptr.To("test-cluster"),
															},
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: `"cluster" must not be set in variable overrides when failureDomain is configured`,
		},
		{
			name:          "Worker: failureDomain configured, variable overrides have subnets - should error",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name:          "worker-1",
									FailureDomain: ptr.To("fd1"),
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															Subnets: []capxv1.NutanixResourceIdentifier{
																{
																	Type: capxv1.NutanixIdentifierName,
																	Name: ptr.To("test-subnet"),
																},
															},
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: `"subnets" must not be set in variable overrides when failureDomain is configured`,
		},
		{
			name:          "Worker: failureDomain configured, machineDetails clean - should pass",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name:          "worker-1",
									FailureDomain: ptr.To("fd1"),
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															VCPUsPerSocket: 2,
															VCPUSockets:    2,
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:          "Worker: no failureDomain, base config has cluster/subnets, override only has other fields - should pass",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Variables: []clusterv1.ClusterVariable{
							{
								Name: "workerConfig",
								Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
									Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
										MachineDetails: v1alpha1.NutanixMachineDetails{
											Cluster: &capxv1.NutanixResourceIdentifier{
												Type: capxv1.NutanixIdentifierName,
												Name: ptr.To("base-cluster"),
											},
											Subnets: []capxv1.NutanixResourceIdentifier{
												{
													Type: capxv1.NutanixIdentifierName,
													Name: ptr.To("base-subnet"),
												},
											},
										},
									},
								}),
							},
						},
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "worker-1",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															VCPUsPerSocket: 2,
															VCPUSockets:    2,
															// Note: no cluster/subnets in override - should inherit from base
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:          "Worker: no failureDomain, machineDetails missing cluster - should error",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "worker-1",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															Subnets: []capxv1.NutanixResourceIdentifier{
																{
																	Type: capxv1.NutanixIdentifierName,
																	Name: ptr.To("test-subnet"),
																},
															},
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: `"cluster" must be set in either base workerConfig or variable overrides when failureDomain is not configured`,
		},
		{
			name:          "Worker: no failureDomain, machineDetails missing subnets - should error",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "worker-1",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															Cluster: &capxv1.NutanixResourceIdentifier{
																Type: capxv1.NutanixIdentifierName,
																Name: ptr.To("test-cluster"),
															},
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: `"subnets" must be set in either base workerConfig or variable overrides when failureDomain is not configured`,
		},
		{
			name:          "Worker: no failureDomain, machineDetails complete - should pass",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name: "worker-1",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															Cluster: &capxv1.NutanixResourceIdentifier{
																Type: capxv1.NutanixIdentifierName,
																Name: ptr.To("test-cluster"),
															},
															Subnets: []capxv1.NutanixResourceIdentifier{
																{
																	Type: capxv1.NutanixIdentifierName,
																	Name: ptr.To("test-subnet"),
																},
															},
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:          "Worker: failureDomain configured, no variable overrides - should pass",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name:          "worker-1",
									FailureDomain: ptr.To("fd1"),
									// No Variables.Overrides - uses worker class as-is
									// Even if class has cluster/subnets, failure domain takes precedence
								},
							},
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name:          "Mixed scenario: some workers with failureDomain, some without - should pass",
			clusterConfig: &variables.ClusterConfigSpec{},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Workers: &clusterv1.WorkersTopology{
							MachineDeployments: []clusterv1.MachineDeploymentTopology{
								{
									Name:          "worker-with-fd",
									FailureDomain: ptr.To("fd1"),
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															VCPUsPerSocket: 2,
															VCPUSockets:    2,
														},
													},
												}),
											},
										},
									},
								},
								{
									Name: "worker-without-fd",
									Variables: &clusterv1.MachineDeploymentVariables{
										Overrides: []clusterv1.ClusterVariable{
											{
												Name: "workerConfig",
												Value: mustMarshalJSON(variables.WorkerNodeConfigSpec{
													Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
														MachineDetails: v1alpha1.NutanixMachineDetails{
															Cluster: &capxv1.NutanixResourceIdentifier{
																Type: capxv1.NutanixIdentifierName,
																Name: ptr.To("test-cluster"),
															},
															Subnets: []capxv1.NutanixResourceIdentifier{
																{
																	Type: capxv1.NutanixIdentifierName,
																	Name: ptr.To("test-subnet"),
																},
															},
														},
													},
												}),
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTopologyFailureDomainConfig(tt.clusterConfig, tt.cluster)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
