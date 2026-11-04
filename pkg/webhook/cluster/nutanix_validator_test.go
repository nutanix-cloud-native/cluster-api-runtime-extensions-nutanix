// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/capx/nutanixprojectpolicy"
)

func TestProjectPolicyAnnotationWidened(t *testing.T) {
	t.Parallel()

	const (
		defaultOnly   = nutanixprojectpolicy.PolicyDefaultOnly
		unrestricted  = nutanixprojectpolicy.PolicyUnrestricted
		singleProject = nutanixprojectpolicy.PolicySingleProject
	)

	clusterWithPolicy := func(value string, moving bool) *clusterv1beta2.Cluster {
		c := &clusterv1beta2.Cluster{}
		annotations := map[string]string{}
		if value != "" {
			annotations[nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey] = value
		}
		if moving {
			annotations[clusterctlv1.DeleteForMoveAnnotation] = ""
		}
		if len(annotations) > 0 {
			c.Annotations = annotations
		}
		return c
	}

	tests := []struct {
		name      string
		oldPolicy string
		newPolicy string
		// moving marks the new object as carrying clusterctl's delete-for-move annotation, i.e. the source
		// object being stamped during a `clusterctl move` pivot.
		moving  bool
		widened bool
	}{
		{name: "narrow unrestricted->default-only", oldPolicy: unrestricted, newPolicy: defaultOnly},
		{name: "absent->default-only", oldPolicy: "", newPolicy: defaultOnly},
		{name: "absent->unrestricted", oldPolicy: "", newPolicy: unrestricted},
		{name: "default-only unchanged", oldPolicy: defaultOnly, newPolicy: defaultOnly},
		{name: "unrestricted unchanged", oldPolicy: unrestricted, newPolicy: unrestricted},
		{name: "widen default-only->unrestricted", oldPolicy: defaultOnly, newPolicy: unrestricted, widened: true},
		{name: "widen default-only->single-project", oldPolicy: defaultOnly, newPolicy: singleProject, widened: true},
		{name: "widen default-only->absent (deletion)", oldPolicy: defaultOnly, newPolicy: "", widened: true},
		{name: "widen default-only->garbage", oldPolicy: defaultOnly, newPolicy: "something-else", widened: true},
		{name: "single-project unchanged", oldPolicy: singleProject, newPolicy: singleProject},
		{name: "narrow unrestricted->single-project", oldPolicy: unrestricted, newPolicy: singleProject},
		{name: "absent->single-project", oldPolicy: "", newPolicy: singleProject},
		{name: "widen single-project->unrestricted", oldPolicy: singleProject, newPolicy: unrestricted, widened: true},
		{name: "widen single-project->default-only", oldPolicy: singleProject, newPolicy: defaultOnly, widened: true},
		{name: "widen single-project->absent (deletion)", oldPolicy: singleProject, newPolicy: "", widened: true},
		{name: "widen single-project->garbage", oldPolicy: singleProject, newPolicy: "something-else", widened: true},
		// clusterctl move replaces the whole annotations map on the source object (dropping project-policy)
		// while adding delete-for-move; these transitions must be exempt so the pivot can proceed.
		{
			name:      "clusterctl move default-only->absent is exempt",
			oldPolicy: defaultOnly,
			newPolicy: "",
			moving:    true,
		},
		{
			name:      "clusterctl move default-only->unrestricted is exempt",
			oldPolicy: defaultOnly,
			newPolicy: unrestricted,
			moving:    true,
		},
		{
			name:      "clusterctl move single-project->absent is exempt",
			oldPolicy: singleProject,
			newPolicy: "",
			moving:    true,
		},
		{
			name:      "clusterctl move single-project->unrestricted is exempt",
			oldPolicy: singleProject,
			newPolicy: unrestricted,
			moving:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				tt.widened,
				projectPolicyAnnotationWidened(
					clusterWithPolicy(tt.oldPolicy, false),
					clusterWithPolicy(tt.newPolicy, tt.moving),
				),
			)
		})
	}
}

func TestStorageProjectPolicyWidened(t *testing.T) {
	t.Parallel()

	const (
		projectPolicy = nutanixprojectpolicy.StoragePolicyProjectPolicy
		defaultPolicy = nutanixprojectpolicy.StoragePolicyDefault
	)

	clusterWith := func(value string, moving bool) *clusterv1beta2.Cluster {
		c := &clusterv1beta2.Cluster{}
		annotations := map[string]string{}
		if value != "" {
			annotations[nutanixprojectpolicy.CSIStorageProjectPolicyAnnotationKey] = value
		}
		if moving {
			annotations[clusterctlv1.DeleteForMoveAnnotation] = ""
		}
		if len(annotations) > 0 {
			c.Annotations = annotations
		}
		return c
	}

	tests := []struct {
		name      string
		oldPolicy string
		newPolicy string
		moving    bool
		widened   bool
	}{
		{name: "project-policy unchanged", oldPolicy: projectPolicy, newPolicy: projectPolicy},
		{name: "default unchanged", oldPolicy: defaultPolicy, newPolicy: defaultPolicy},
		{name: "narrow default->project-policy", oldPolicy: defaultPolicy, newPolicy: projectPolicy},
		{name: "narrow Default->project-policy", oldPolicy: "Default", newPolicy: projectPolicy},
		{name: "absent->project-policy", oldPolicy: "", newPolicy: projectPolicy},
		{name: "absent->default", oldPolicy: "", newPolicy: defaultPolicy},
		{name: "widen project-policy->absent", oldPolicy: projectPolicy, newPolicy: "", widened: true},
		{name: "widen project-policy->default", oldPolicy: projectPolicy, newPolicy: defaultPolicy, widened: true},
		{name: "widen Project-Policy->DEFAULT", oldPolicy: "Project-Policy", newPolicy: "DEFAULT", widened: true},
		{name: "widen project-policy->garbage", oldPolicy: projectPolicy, newPolicy: "something-else", widened: true},
		{
			name:      "clusterctl move project-policy->default is exempt",
			oldPolicy: projectPolicy,
			newPolicy: defaultPolicy,
			moving:    true,
		},
		{
			name:      "clusterctl move project-policy->absent is exempt",
			oldPolicy: projectPolicy,
			newPolicy: "",
			moving:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				tt.widened,
				storageProjectPolicyWidened(
					clusterWith(tt.oldPolicy, false),
					clusterWith(tt.newPolicy, tt.moving),
				),
			)
		})
	}
}

func TestProjectUUIDAnnotationChanged(t *testing.T) {
	t.Parallel()

	const (
		singleProject = nutanixprojectpolicy.PolicySingleProject
		unrestricted  = nutanixprojectpolicy.PolicyUnrestricted
		uuidA         = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		uuidB         = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)

	clusterWith := func(policy, uuid string, moving bool) *clusterv1beta2.Cluster {
		c := &clusterv1beta2.Cluster{}
		annotations := map[string]string{}
		if policy != "" {
			annotations[nutanixprojectpolicy.NutanixProjectPolicyAnnotationKey] = policy
		}
		if uuid != "" {
			annotations[nutanixprojectpolicy.NutanixProjectUUIDAnnotationKey] = uuid
		}
		if moving {
			annotations[clusterctlv1.DeleteForMoveAnnotation] = ""
		}
		if len(annotations) > 0 {
			c.Annotations = annotations
		}
		return c
	}

	tests := []struct {
		name      string
		oldPolicy string
		oldUUID   string
		newPolicy string
		newUUID   string
		moving    bool
		changed   bool
	}{
		{
			name:      "single-project uuid unchanged",
			oldPolicy: singleProject,
			oldUUID:   uuidA,
			newPolicy: singleProject,
			newUUID:   uuidA,
		},
		{
			name:      "single-project uuid case-insensitive match is unchanged",
			oldPolicy: singleProject,
			oldUUID:   uuidA,
			newPolicy: singleProject,
			newUUID:   "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA",
		},
		{
			name:      "single-project uuid swap is a change",
			oldPolicy: singleProject,
			oldUUID:   uuidA,
			newPolicy: singleProject,
			newUUID:   uuidB,
			changed:   true,
		},
		{
			name:      "single-project uuid deletion is a change",
			oldPolicy: singleProject,
			oldUUID:   uuidA,
			newPolicy: singleProject,
			newUUID:   "",
			changed:   true,
		},
		{
			name:      "stamp uuid onto single-project is not a change",
			oldPolicy: singleProject,
			oldUUID:   "",
			newPolicy: singleProject,
			newUUID:   uuidA,
		},
		{
			name:      "unrestricted to single-project with uuid is not a change",
			oldPolicy: unrestricted,
			newPolicy: singleProject,
			newUUID:   uuidA,
		},
		{
			name:      "clusterctl move may drop single-project uuid",
			oldPolicy: singleProject,
			oldUUID:   uuidA,
			newPolicy: "",
			newUUID:   "",
			moving:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				tt.changed,
				projectUUIDAnnotationChanged(
					clusterWith(tt.oldPolicy, tt.oldUUID, false),
					clusterWith(tt.newPolicy, tt.newUUID, tt.moving),
				),
			)
		})
	}
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
	testcases := []struct {
		name                string
		clusterConfig       *variables.ClusterConfigSpec
		cluster             *clusterv1beta2.Cluster
		expectedErr         bool
		expectedErrMessages []string
	}{
		{
			name:          "controlPlane failureDomains not configured and missing machineDetail cluster",
			clusterConfig: fakeClusterConfigSpec(false, false, true),
			cluster:       fakeCluster(t, true, false, false, workerConfigNone),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.cluster: Required value: \"cluster\" must be set when failureDomains are not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:          "controlPlane failureDomains not configured and missing machineDetail subnets",
			clusterConfig: fakeClusterConfigSpec(false, true, false),
			cluster:       fakeCluster(t, true, false, false, workerConfigNone),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.subnets: Required value: \"subnets\" must be set when failureDomains are not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:          "controlPlane failureDomains not configured and missing machineDetail cluster and subnets",
			clusterConfig: fakeClusterConfigSpec(false, false, false),
			cluster:       fakeCluster(t, true, false, false, workerConfigNone),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.cluster: Required value: \"cluster\" must be set when failureDomains are not configured.", //nolint:lll // Message is long.
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.subnets: Required value: \"subnets\" must be set when failureDomains are not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:                "controlPlane failureDomains configured and missing machineDetail cluster and subnets",
			clusterConfig:       fakeClusterConfigSpec(true, false, false),
			cluster:             fakeCluster(t, true, false, false, workerConfigNone),
			expectedErr:         false,
			expectedErrMessages: nil,
		},
		{
			name:          "worker failureDomain not configured and missing 'cluster' in workerConfig overrides only",
			clusterConfig: fakeClusterConfigSpec(true, false, false),
			cluster:       fakeCluster(t, false, false, true, workerConfigOverridesOnly),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: Required value: \"cluster\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:          "worker failureDomain not configured and missing machineDetail 'subnets' in workerConfig overrides only", //nolint:lll // name is long.
			clusterConfig: fakeClusterConfigSpec(true, false, false),
			cluster:       fakeCluster(t, false, true, false, workerConfigOverridesOnly),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: Required value: \"subnets\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:          "worker failureDomain not configured and missing machineDetail 'cluster' and 'subnets' in workerConfig overrides only", //nolint:lll // name is long.
			clusterConfig: fakeClusterConfigSpec(true, false, false),
			cluster:       fakeCluster(t, false, false, false, workerConfigOverridesOnly),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: Required value: \"cluster\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: Required value: \"subnets\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:                "worker failureDomain configured and missing machineDetail cluster and subnets in workerConfig overrides only", //nolint:lll // name is long.
			clusterConfig:       fakeClusterConfigSpec(true, false, false),
			cluster:             fakeCluster(t, true, false, false, workerConfigOverridesOnly),
			expectedErr:         false,
			expectedErrMessages: nil,
		},
		{
			name:          "worker failureDomain not configured and missing machineDetail 'cluster' and 'subnets' in workerConfig default only", //nolint:lll // name is long.
			clusterConfig: fakeClusterConfigSpec(true, false, false),
			cluster:       fakeCluster(t, false, false, false, workerConfigDefaultOnly),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.variables.workerConfig.value.nutanix.machineDetails.cluster: Required value: \"cluster\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
				"spec.topology.variables.workerConfig.value.nutanix.machineDetails.subnets: Required value: \"subnets\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:          "worker failureDomain not configured and missing machineDetail 'cluster' and 'subnets' in workerConfig default and overrides both", //nolint:lll // name is long.
			clusterConfig: fakeClusterConfigSpec(true, false, false),
			cluster:       fakeCluster(t, false, false, false, workerConfigDefaultOverridesBoth),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: Required value: \"cluster\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: Required value: \"subnets\" must be set when failureDomain is not configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:                "worker failureDomain configured and missing machineDetail cluster and subnets in workerConfig default and overrides both", //nolint:lll // name is long.
			clusterConfig:       fakeClusterConfigSpec(true, false, false),
			cluster:             fakeCluster(t, true, false, false, workerConfigDefaultOverridesBoth),
			expectedErr:         false,
			expectedErrMessages: nil,
		},
		{
			name:                "worker failureDomain configured and missing workerConfig",
			clusterConfig:       fakeClusterConfigSpec(true, false, false),
			cluster:             fakeCluster(t, true, false, false, workerConfigNone),
			expectedErr:         false,
			expectedErrMessages: nil,
		},
		{
			name:          "controlPlane failureDomains configured with cluster/subnets set violates XOR",
			clusterConfig: fakeClusterConfigSpec(true, true, true),
			cluster:       fakeCluster(t, true, false, false, workerConfigNone),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.cluster: Forbidden: \"cluster\" must not be set when failureDomains are configured.", //nolint:lll // Message is long.
				"spec.topology.variables.clusterConfig.value.controlPlane.nutanix.machineDetails.subnets: Forbidden: \"subnets\" must not be set when failureDomains are configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:          "worker failureDomain configured with cluster/subnets set in variables overrides violates XOR",
			clusterConfig: fakeClusterConfigSpec(true, false, false),
			cluster:       fakeCluster(t, true, true, true, workerConfigOverridesOnly),
			expectedErr:   true,
			expectedErrMessages: []string{
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.cluster: Forbidden: \"cluster\" must not be set when failureDomain is configured.", //nolint:lll // Message is long.
				"spec.topology.workers.machineDeployments.variables.overrides.workerConfig.value.nutanix.machineDetails.subnets: Forbidden: \"subnets\" must not be set when failureDomain is configured.", //nolint:lll // Message is long.
			},
		},
		{
			name:                "default workerConfig variable with cluster/subnets set when control plane has failureDomains should be allowed", //nolint:lll // name is long.
			clusterConfig:       fakeClusterConfigSpec(true, false, false),
			cluster:             fakeCluster(t, false, true, true, workerConfigDefaultOnly),
			expectedErr:         false,
			expectedErrMessages: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTopologyFailureDomainConfig(tc.clusterConfig, tc.cluster)
			assert.Equal(t, tc.expectedErr, err != nil)
			if tc.expectedErr && len(tc.expectedErrMessages) > 0 {
				for _, errMsg := range tc.expectedErrMessages {
					assert.ErrorContains(t, err, errMsg)
				}
			}
		})
	}
}

func fakeMachineDetails(pe, subnets bool) *v1alpha1.NutanixMachineDetails {
	md := &v1alpha1.NutanixMachineDetails{
		BootType:       capxv1.NutanixBootTypeLegacy,
		VCPUSockets:    2,
		VCPUsPerSocket: 1,
		Image: &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-image"),
		},
		MemorySize:     resource.MustParse("8Gi"),
		SystemDiskSize: resource.MustParse("40Gi"),
	}

	if pe {
		md.Cluster = &capxv1.NutanixResourceIdentifier{
			Type: capxv1.NutanixIdentifierName,
			Name: new("fake-pe-cluster"),
		}
	}

	if subnets {
		md.Subnets = []capxv1.NutanixResourceIdentifier{
			{
				Type: capxv1.NutanixIdentifierName,
				Name: new("fake-subnet"),
			},
		}
	}

	return md
}

func fakeClusterConfigSpec(fd, pe, subnets bool) *variables.ClusterConfigSpec {
	clusterCfg := &variables.ClusterConfigSpec{
		ControlPlane: &variables.ControlPlaneSpec{
			Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
				MachineDetails: *fakeMachineDetails(pe, subnets),
			},
		},
	}

	if fd {
		clusterCfg.ControlPlane.Nutanix.FailureDomains = []string{"fd-1", "fd-2", "fd-3"}
	}

	return clusterCfg
}

type workerConfigExistence string

const (
	workerConfigDefaultOnly          workerConfigExistence = "workerConfigDefaultOnly"
	workerConfigOverridesOnly        workerConfigExistence = "workerConfigOverridesOnly"
	workerConfigDefaultOverridesBoth workerConfigExistence = "workerConfigDefaultOverridesBoth"
	workerConfigNone                 workerConfigExistence = "workerConfigNone"
)

func fakeCluster(t *testing.T, fd, pe, subnets bool, wcfg workerConfigExistence) *clusterv1beta2.Cluster {
	t.Helper()

	cluster := &clusterv1beta2.Cluster{
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				Variables: []clusterv1beta2.ClusterVariable{},
				Workers: clusterv1beta2.WorkersTopology{
					MachineDeployments: []clusterv1beta2.MachineDeploymentTopology{
						{
							Class: "default-worker",
							Name:  "md-1",
							Variables: clusterv1beta2.MachineDeploymentVariables{
								Overrides: []clusterv1beta2.ClusterVariable{},
							},
						},
					},
				},
			},
		},
	}

	if fd {
		cluster.Spec.Topology.Workers.MachineDeployments[0].FailureDomain = "fd-1"
	}

	workerConfig := &variables.WorkerNodeConfigSpec{
		Nutanix: &v1alpha1.NutanixWorkerNodeSpec{
			MachineDetails: *fakeMachineDetails(pe, subnets),
		},
	}
	workerConfigVar, err := variables.MarshalToClusterVariable("workerConfig", workerConfig)
	assert.NoError(t, err)

	switch wcfg {
	case workerConfigDefaultOnly:
		cluster.Spec.Topology.Variables = append(cluster.Spec.Topology.Variables, *workerConfigVar)
	case workerConfigOverridesOnly:
		cluster.Spec.Topology.Workers.MachineDeployments[0].Variables.Overrides = append(
			cluster.Spec.Topology.Workers.MachineDeployments[0].Variables.Overrides,
			*workerConfigVar,
		)
	case workerConfigDefaultOverridesBoth:
		cluster.Spec.Topology.Variables = append(cluster.Spec.Topology.Variables, *workerConfigVar)
		cluster.Spec.Topology.Workers.MachineDeployments[0].Variables.Overrides = append(
			cluster.Spec.Topology.Workers.MachineDeployments[0].Variables.Overrides,
			*workerConfigVar,
		)
	case workerConfigNone:
	default:
		break
	}

	return cluster
}
