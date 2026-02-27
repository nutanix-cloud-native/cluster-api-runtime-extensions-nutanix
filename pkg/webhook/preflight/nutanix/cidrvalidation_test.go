// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net/netip"
	"testing"

	networkingcommonapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/common/v1/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type expectedCause struct {
	messagePart string
	field       string
}

func TestNewCIDRValidationChecks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cd             *checkDependencies
		expectedChecks int
	}{
		{
			name:           "nil dependencies",
			cd:             nil,
			expectedChecks: 0,
		},
		{
			name:           "missing cluster",
			cd:             &checkDependencies{},
			expectedChecks: 0,
		},
		{
			name: "cluster without nclient creates check",
			cd: &checkDependencies{
				cluster: testCluster([]string{"10.244.0.0/16"}, []string{"10.96.0.0/12"}),
			},
			expectedChecks: 1,
		},
		{
			name: "full dependencies",
			cd: &checkDependencies{
				cluster:   testCluster([]string{"10.244.0.0/16"}, []string{"10.96.0.0/12"}),
				nclient:   &clientWrapper{},
				pcVersion: "7.3.0",
			},
			expectedChecks: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checks := newCIDRValidationChecks(tt.cd)
			require.Len(t, checks, tt.expectedChecks)
			if tt.expectedChecks > 0 {
				assert.Equal(t, "NutanixCIDRValidation", checks[0].Name())
			}
		})
	}
}

func TestCIDRValidationCheckRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		podCIDRs             []string
		serviceCIDRs         []string
		resolvedNodeSubnets  []resolvedNodeSubnet
		resolveWarnings      []string
		resolveSubnetErr     error
		withConfiguredSubnet bool
		withNClient          bool
		expectAllowed        bool
		expectInternalError  bool
		expectedCauses       []expectedCause
		expectedWarnings     int
	}{
		{
			name:             "valid non-overlapping CIDRs with large ranges",
			podCIDRs:         []string{"10.244.0.0/16"},
			serviceCIDRs:     []string{"10.96.0.0/12"},
			expectAllowed:    true,
			expectedWarnings: 0,
		},
		{
			name:          "pod CIDR /24 blocks deployment",
			podCIDRs:      []string{"10.244.0.0/24"},
			serviceCIDRs:  []string{"10.96.0.0/12"},
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "Pod CIDR \"10.244.0.0/24\" has prefix /24, which is too small for multi-node clusters",
					field:       "$.spec.clusterNetwork.pods.cidrBlocks",
				},
			},
			expectedWarnings: 0,
		},
		{
			name:             "pod CIDR /22 warns but allows",
			podCIDRs:         []string{"10.244.0.0/22"},
			serviceCIDRs:     []string{"10.96.0.0/12"},
			expectAllowed:    true,
			expectedWarnings: 1,
		},
		{
			name:          "service CIDR /24 blocks deployment",
			podCIDRs:      []string{"10.244.0.0/16"},
			serviceCIDRs:  []string{"10.96.0.0/24"},
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "Service CIDR \"10.96.0.0/24\" is too small",
					field:       "$.spec.clusterNetwork.services.cidrBlocks",
				},
			},
			expectedWarnings: 0,
		},
		{
			name:          "service CIDR /25 blocks deployment",
			podCIDRs:      []string{"10.244.0.0/16"},
			serviceCIDRs:  []string{"10.96.0.0/25"},
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "Service CIDR \"10.96.0.0/25\" is too small",
					field:       "$.spec.clusterNetwork.services.cidrBlocks",
				},
			},
			expectedWarnings: 0,
		},
		{
			name:             "service CIDR /22 warns but allows",
			podCIDRs:         []string{"10.244.0.0/16"},
			serviceCIDRs:     []string{"10.96.0.0/22"},
			expectAllowed:    true,
			expectedWarnings: 1,
		},
		{
			name:             "service CIDR /20 passes without warning",
			podCIDRs:         []string{"10.244.0.0/16"},
			serviceCIDRs:     []string{"10.96.0.0/20"},
			expectAllowed:    true,
			expectedWarnings: 0,
		},
		{
			name:             "pod service overlap",
			podCIDRs:         []string{"10.96.0.0/16"},
			serviceCIDRs:     []string{"10.96.0.0/12"},
			expectAllowed:    false,
			expectedWarnings: 0,
			expectedCauses: []expectedCause{
				{
					messagePart: "overlaps with Service CIDR",
					field:       "$.spec.clusterNetwork.pods.cidrBlocks",
				},
			},
		},
		{
			name:         "pod and service overlap with node subnet",
			podCIDRs:     []string{"10.244.0.0/16"},
			serviceCIDRs: []string{"10.96.0.0/12"},
			resolvedNodeSubnets: []resolvedNodeSubnet{
				{
					prefix: netip.MustParsePrefix("10.244.128.0/24"),
					field:  "$.spec.topology.variables[?@.name==\"clusterConfig\"].value.controlPlane.nutanix.machineDetails.subnets[0]", //nolint:lll // Field path is long.
					name:   "vlan173",
				},
				{
					prefix: netip.MustParsePrefix("10.100.0.0/16"),
					field:  "$.spec.topology.workers.machineDeployments[?@.name==\"md-0\"].variables[?@.name=workerConfig].value.nutanix.machineDetails.subnets[0]", //nolint:lll // Field path is long.
					name:   "worker-subnet",
				},
			},
			withConfiguredSubnet: true,
			withNClient:          true,
			expectAllowed:        false,
			expectedWarnings:     0,
			expectedCauses: []expectedCause{
				{
					messagePart: `Pod CIDR "10.244.0.0/16" overlaps with node subnet "vlan173"`,
					field:       `$.spec.topology.variables[?@.name=="clusterConfig"].value.controlPlane.nutanix.machineDetails.subnets[0]`,
				},
				{
					messagePart: `Service CIDR "10.96.0.0/12" overlaps with node subnet "worker-subnet"`,
					field:       `$.spec.topology.workers.machineDeployments[?@.name=="md-0"].variables[?@.name=workerConfig].value.nutanix.machineDetails.subnets[0]`,
				},
			},
		},
		{
			name:                 "subnet resolution error",
			podCIDRs:             []string{"10.244.0.0/16"},
			serviceCIDRs:         []string{"10.96.0.0/12"},
			resolveSubnetErr:     fmt.Errorf("temporary prism error"),
			withConfiguredSubnet: true,
			withNClient:          true,
			expectAllowed:        false,
			expectInternalError:  true,
			expectedWarnings:     0,
			expectedCauses: []expectedCause{
				{
					messagePart: "Failed to resolve node subnet CIDRs",
					field:       "",
				},
			},
		},
		{
			name:                 "missing nclient with configured subnets returns early without error",
			podCIDRs:             []string{"10.244.0.0/16"},
			serviceCIDRs:         []string{"10.96.0.0/12"},
			withConfiguredSubnet: true,
			withNClient:          false,
			expectAllowed:        true,
			expectedWarnings:     0,
		},
		{
			name:          "invalid pod cidr",
			podCIDRs:      []string{"not-a-cidr"},
			serviceCIDRs:  []string{"10.96.0.0/12"},
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "Invalid Pod CIDR configuration",
					field:       "$.spec.clusterNetwork.pods.cidrBlocks",
				},
			},
		},
		{
			name:          "invalid service cidr",
			podCIDRs:      []string{"10.244.0.0/16"},
			serviceCIDRs:  []string{"not-a-cidr"},
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "Invalid Service CIDR configuration",
					field:       "$.spec.clusterNetwork.services.cidrBlocks",
				},
			},
		},
		{
			name:          "both invalid pod and service cidrs reported together",
			podCIDRs:      []string{"not-a-cidr"},
			serviceCIDRs:  []string{"also-not-a-cidr"},
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "Invalid Pod CIDR configuration",
					field:       "$.spec.clusterNetwork.pods.cidrBlocks",
				},
				{
					messagePart: "Invalid Service CIDR configuration",
					field:       "$.spec.clusterNetwork.services.cidrBlocks",
				},
			},
		},
		{
			name:                "external IPAM subnet produces warning and skips overlap",
			podCIDRs:            []string{"10.244.0.0/16"},
			serviceCIDRs:        []string{"10.96.0.0/12"},
			resolvedNodeSubnets: nil,
			resolveWarnings: []string{
				"Subnet \"ext-ipam\" appears to use external IPAM (no IP configuration found). CIDR overlap validation will be skipped for this subnet.",
			},
			withConfiguredSubnet: true,
			withNClient:          true,
			expectAllowed:        true,
			expectedWarnings:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cd := &checkDependencies{
				cluster: testCluster(tt.podCIDRs, tt.serviceCIDRs),
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					ControlPlane: &carenv1.NutanixControlPlaneSpec{
						Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
							MachineDetails: carenv1.NutanixMachineDetails{
								Subnets: configuredSubnets(tt.withConfiguredSubnet),
							},
						},
					},
				},
			}

			if tt.withNClient {
				cd.nclient = &clientWrapper{}
			}

			check := &cidrValidationCheck{
				cd: cd,
				resolveNodeSubnetsFunc: func(
					_ context.Context,
					_ client,
					sources []nodeSubnetSource,
				) ([]resolvedNodeSubnet, []string, error) {
					if !tt.withConfiguredSubnet {
						require.Empty(t, sources)
					} else {
						require.NotEmpty(t, sources)
					}
					if tt.resolveSubnetErr != nil {
						return nil, nil, tt.resolveSubnetErr
					}
					return tt.resolvedNodeSubnets, tt.resolveWarnings, nil
				},
			}

			result := check.Run(context.Background())

			assert.Equal(t, tt.expectAllowed, result.Allowed, "Allowed mismatch")
			assert.Equal(t, tt.expectInternalError, result.InternalError, "InternalError mismatch")
			assert.Len(t, result.Warnings, tt.expectedWarnings, "Warnings count mismatch")

			require.Len(t, result.Causes, len(tt.expectedCauses), "Causes count mismatch")
			for i, expected := range tt.expectedCauses {
				assert.Contains(t, result.Causes[i].Message, expected.messagePart,
					"cause[%d] message mismatch", i)
				assert.Equal(t, expected.field, result.Causes[i].Field,
					"cause[%d] field mismatch", i)
			}
		})
	}
}

func TestCollectNodeSubnetSources(t *testing.T) {
	t.Parallel()

	subnetByName := capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: ptr.To("subnet-a"),
	}
	subnetByUUID := capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: ptr.To("11111111-1111-1111-1111-111111111111"),
	}

	cd := &checkDependencies{
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					MachineDetails: carenv1.NutanixMachineDetails{
						Subnets: []capxv1.NutanixResourceIdentifier{
							subnetByName,
							subnetByUUID,
						},
					},
				},
			},
		},
		nutanixWorkerNodeConfigSpecByMachineDeploymentName: map[string]*carenv1.NutanixWorkerNodeConfigSpec{
			"md-1": {
				Nutanix: &carenv1.NutanixWorkerNodeSpec{
					MachineDetails: carenv1.NutanixMachineDetails{
						Subnets: []capxv1.NutanixResourceIdentifier{
							subnetByName, // same subnet, but different source - both kept
						},
					},
				},
			},
		},
	}

	sources := collectNodeSubnetSources(cd)
	// 2 from control plane + 1 from worker = 3 (no dedup, sources are distinct)
	require.Len(t, sources, 3)

	assert.Equal(t,
		`$.spec.topology.variables[?@.name=="clusterConfig"].value.controlPlane.nutanix.machineDetails.subnets[0]`,
		sources[0].field,
	)
	assert.Equal(t,
		`$.spec.topology.variables[?@.name=="clusterConfig"].value.controlPlane.nutanix.machineDetails.subnets[1]`,
		sources[1].field,
	)
	assert.Equal(
		t,
		`$.spec.topology.workers.machineDeployments[?@.name=="md-1"].variables[?@.name=workerConfig].value.nutanix.machineDetails.subnets[0]`,
		sources[2].field,
	)
}

func TestExtractIPv4PrefixesFromSubnet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		subnet          *netv4.Subnet
		expectedCount   int
		expectNilResult bool
		expectError     bool
	}{
		{
			name:            "empty IpConfig returns nil (external IPAM)",
			subnet:          &netv4.Subnet{},
			expectedCount:   0,
			expectNilResult: true,
			expectError:     false,
		},
		{
			name: "IpConfig with only IPv6 returns empty slice",
			subnet: &netv4.Subnet{
				IpConfig: []netv4.IPConfig{
					{
						Ipv6: &netv4.IPv6Config{},
					},
				},
			},
			expectedCount:   0,
			expectNilResult: false,
			expectError:     false,
		},
		{
			name: "IpConfig with valid IPv4 returns prefix",
			subnet: &netv4.Subnet{
				IpConfig: []netv4.IPConfig{
					{
						Ipv4: &netv4.IPv4Config{
							IpSubnet: &netv4.IPv4Subnet{
								Ip: &networkingcommonapi.IPv4Address{
									Value: ptr.To("10.0.0.0"),
								},
								PrefixLength: ptr.To(24),
							},
						},
					},
				},
			},
			expectedCount:   1,
			expectNilResult: false,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			prefixes, err := extractIPv4PrefixesFromSubnet(tt.subnet)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.expectNilResult {
				assert.Nil(t, prefixes)
			} else {
				assert.Len(t, prefixes, tt.expectedCount)
			}
		})
	}
}

func testCluster(podCIDRs, serviceCIDRs []string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: podCIDRs,
				},
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: serviceCIDRs,
				},
			},
		},
	}
}

func configuredSubnets(enabled bool) []capxv1.NutanixResourceIdentifier {
	if !enabled {
		return nil
	}
	return []capxv1.NutanixResourceIdentifier{
		{
			Type: capxv1.NutanixIdentifierName,
			Name: ptr.To("subnet-a"),
		},
	}
}
