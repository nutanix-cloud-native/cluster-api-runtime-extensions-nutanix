// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

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
	scenarios := []testScenario{
		{
			name: "controlPlane failureDomains not configured and machineDetail cluster and subnets not set",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: false,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     nil,
				overrideConfig:    nil,
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.cluster: " +
					"Required value: 'cluster' must be set when failureDomains are not configured.",
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.subnets: " +
					"Required value: 'subnets' must be set when failureDomains are not configured.",
			},
		},
		{
			name: "controlPlane failureDomains configured and machineDetail cluster and subnets set",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        true,
				hasSubnets:        true,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     nil,
				overrideConfig:    nil,
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.cluster: " +
					"Forbidden: 'cluster' must not be set when failureDomains are configured.",
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.subnets: " +
					"Forbidden: 'subnets' must not be set when failureDomains are configured.",
			},
		},
		{
			name: "controlPlane failureDomains configured and machineDetail cluster and subnets not set",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     nil,
				overrideConfig:    nil,
			},
			expectedErr: false,
		},
		{
			name: "worker failureDomain not configured and machineDetail 'cluster' and 'subnets' " +
				"not set in workerConfig overrides only",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: false},
				defaultConfig:     nil,
				overrideConfig:    &machineDetailsConfig{hasCluster: false, hasSubnets: false},
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: " +
					"Required value: 'cluster' must be set when failureDomain is not configured.",
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: " +
					"Required value: 'subnets' must be set when failureDomain is not configured.",
			},
		},
		{
			name: "worker failureDomain configured and machineDetail cluster and subnets not set in workerConfig overrides only",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     nil,
				overrideConfig:    &machineDetailsConfig{hasCluster: false, hasSubnets: false},
			},
			expectedErr: false,
		},
		{
			name: "worker failureDomain configured and machineDetail cluster and subnets set in workerConfig overrides only",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     nil,
				overrideConfig:    &machineDetailsConfig{hasCluster: true, hasSubnets: true},
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: " +
					"Forbidden: 'cluster' must not be set when failureDomain is configured.",
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: " +
					"Forbidden: 'subnets' must not be set when failureDomain is configured.",
			},
		},
		{
			name: "worker failureDomain not configured and machineDetail 'cluster' and 'subnets' " +
				"not set in workerConfig default only",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: false},
				defaultConfig:     &machineDetailsConfig{hasCluster: false, hasSubnets: false},
				overrideConfig:    nil,
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.variables.workerConfig.value.nutanix.machineDetails.cluster: " +
					"Required value: 'cluster' must be set when failureDomain is not configured.",
				"spec.topology.variables.workerConfig.value.nutanix.machineDetails.subnets: " +
					"Required value: 'subnets' must be set when failureDomain is not configured.",
			},
		},
		{
			name: "worker failureDomain not configured and machineDetail 'cluster' and 'subnets' " +
				"not set in workerConfig default and overrides both",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: false},
				defaultConfig:     &machineDetailsConfig{hasCluster: false, hasSubnets: false},
				overrideConfig:    &machineDetailsConfig{hasCluster: false, hasSubnets: false},
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: " +
					"Required value: 'cluster' must be set when failureDomain is not configured.",
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: " +
					"Required value: 'subnets' must be set when failureDomain is not configured.",
			},
		},
		{
			name: "controlPlane failureDomains configured, MD without failureDomain, no default workerConfig - should fail",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: false},
				defaultConfig:     nil,
				overrideConfig:    nil,
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"'cluster' must be set when failureDomain is not configured.",
				"'subnets' must be set when failureDomain is not configured.",
			},
		},
		{
			name: "worker failureDomain configured with cluster and subnets set in workerConfig default only",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     &machineDetailsConfig{hasCluster: true, hasSubnets: true},
				overrideConfig:    nil,
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"spec.topology.variables.workerConfig.value.nutanix.machineDetails.cluster: " +
					"Forbidden: 'cluster' must not be set when failureDomain is configured.",
				"spec.topology.variables.workerConfig.value.nutanix.machineDetails.subnets: " +
					"Forbidden: 'subnets' must not be set when failureDomain is configured.",
			},
		},
		{
			name: "controlPlane failureDomains configured and default workerConfig has cluster and subnets - cross-validation",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     &machineDetailsConfig{hasCluster: true, hasSubnets: true},
				overrideConfig:    nil,
			},
			expectedErr: true,
			expectedErrMessages: []string{
				"'cluster' must not be set in default workerConfig when control plane has failureDomains configured.",
				"'subnets' must not be set in default workerConfig when control plane has failureDomains configured.",
			},
		},
		{
			name: "controlPlane no failureDomains and default workerConfig has cluster and subnets - cross-validation pass",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: false,
				hasCluster:        true,
				hasSubnets:        true,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: false},
				defaultConfig:     &machineDetailsConfig{hasCluster: true, hasSubnets: true},
				overrideConfig:    nil,
			},
			expectedErr: false,
		},
		{
			name: "controlPlane failureDomains configured, no default workerConfig, " +
				"MD override with cluster/subnets and no failureDomain - should pass",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: false},
				defaultConfig:     nil,
				overrideConfig:    &machineDetailsConfig{hasCluster: true, hasSubnets: true},
			},
			expectedErr: false,
		},
		{
			name: "controlPlane failureDomains configured, MD with failureDomain, no workerConfig - should pass",
			controlPlane: controlPlaneConfig{
				hasFailureDomains: true,
				hasCluster:        false,
				hasSubnets:        false,
			},
			worker: workerConfig{
				machineDeployment: machineDeploymentConfig{hasFailureDomain: true},
				defaultConfig:     nil,
				overrideConfig:    nil,
			},
			expectedErr: false,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			clusterConfig, cluster := scenario.buildTestCluster(t)
			err := validateTopologyFailureDomainConfig(clusterConfig, cluster)

			if scenario.expectedErr {
				require.Error(t, err)
				if len(scenario.expectedErrMessages) > 0 {
					for _, msg := range scenario.expectedErrMessages {
						require.ErrorContains(t, err, msg)
					}
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// testScenario represents a complete test scenario configuration.
type testScenario struct {
	name                string
	controlPlane        controlPlaneConfig
	worker              workerConfig
	expectedErr         bool
	expectedErrMessages []string
}

type controlPlaneConfig struct {
	hasFailureDomains bool
	hasCluster        bool
	hasSubnets        bool
}

type workerConfig struct {
	machineDeployment machineDeploymentConfig
	defaultConfig     *machineDetailsConfig // nil means no default workerConfig
	overrideConfig    *machineDetailsConfig // nil means no override workerConfig
}

type machineDeploymentConfig struct {
	hasFailureDomain bool
}

type machineDetailsConfig struct {
	hasCluster bool
	hasSubnets bool
}

// buildTestCluster creates a complete cluster configuration from a test scenario.
func (ts *testScenario) buildTestCluster(t *testing.T) (*variables.ClusterConfigSpec, *clusterv1.Cluster) {
	t.Helper()

	// Build cluster config spec
	clusterConfig := &variables.ClusterConfigSpec{
		ControlPlane: &variables.ControlPlaneSpec{
			Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
				MachineDetails: *fakeMachineDetails(ts.controlPlane.hasCluster, ts.controlPlane.hasSubnets),
			},
		},
	}

	if ts.controlPlane.hasFailureDomains {
		clusterConfig.ControlPlane.Nutanix.FailureDomains = []string{"fd-1", "fd-2", "fd-3"}
	}

	// Build cluster
	cluster := &clusterv1.Cluster{
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Variables: []clusterv1.ClusterVariable{},
				Workers: &clusterv1.WorkersTopology{
					MachineDeployments: []clusterv1.MachineDeploymentTopology{
						{
							Class: "default-worker",
							Name:  "md-1",
							Variables: &clusterv1.MachineDeploymentVariables{
								Overrides: []clusterv1.ClusterVariable{},
							},
						},
					},
				},
			},
		},
	}

	// Set failure domain on machine deployment if specified
	if ts.worker.machineDeployment.hasFailureDomain {
		cluster.Spec.Topology.Workers.MachineDeployments[0].FailureDomain = ptr.To("fd-1")
	}

	// Add default workerConfig if specified
	if ts.worker.defaultConfig != nil {
		workerConfig := &variables.WorkerNodeConfigSpec{
			Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
				MachineDetails: *fakeMachineDetails(ts.worker.defaultConfig.hasCluster, ts.worker.defaultConfig.hasSubnets),
			},
		}
		workerConfigVar, err := variables.MarshalToClusterVariable("workerConfig", workerConfig)
		require.NoError(t, err)
		cluster.Spec.Topology.Variables = append(cluster.Spec.Topology.Variables, *workerConfigVar)
	}

	// Add override workerConfig if specified
	if ts.worker.overrideConfig != nil {
		workerConfig := &variables.WorkerNodeConfigSpec{
			Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
				MachineDetails: *fakeMachineDetails(ts.worker.overrideConfig.hasCluster, ts.worker.overrideConfig.hasSubnets),
			},
		}
		workerConfigVar, err := variables.MarshalToClusterVariable("workerConfig", workerConfig)
		require.NoError(t, err)
		cluster.Spec.Topology.Workers.MachineDeployments[0].Variables.Overrides = append(
			cluster.Spec.Topology.Workers.MachineDeployments[0].Variables.Overrides,
			*workerConfigVar,
		)
	}

	return clusterConfig, cluster
}

func fakeMachineDetails(pe, subnets bool) *v1alpha1.NutanixMachineDetails {
	md := &v1alpha1.NutanixMachineDetails{
		BootType:       capxv1.NutanixBootTypeLegacy,
		VCPUSockets:    2,
		VCPUsPerSocket: 1,
		Image: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-image"),
		},
		MemorySize:     resource.MustParse("8Gi"),
		SystemDiskSize: resource.MustParse("40Gi"),
	}

	if pe {
		md.Cluster = &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("fake-pe-cluster"),
		}
	}

	if subnets {
		md.Subnets = []capxv1.NutanixResourceIdentifier{
			{
				Type: capxv1.NutanixIdentifierName,
				Name: ptr.To("fake-subnet"),
			},
		}
	}

	return md
}
