// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

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
		resolveSubnetCIDRs   []netip.Prefix
		resolveSubnetErr     error
		withConfiguredSubnet bool
		withNClient          bool
		expectAllowed        bool
		expectInternalError  bool
		expectedCauseParts   []string
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
			expectedCauseParts: []string{
				"Pod CIDR \"10.244.0.0/24\" has prefix /24, which is too small for multi-node clusters",
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
			expectedCauseParts: []string{
				"Service CIDR \"10.96.0.0/24\" is too small",
			},
			expectedWarnings: 0,
		},
		{
			name:          "service CIDR /25 blocks deployment",
			podCIDRs:      []string{"10.244.0.0/16"},
			serviceCIDRs:  []string{"10.96.0.0/25"},
			expectAllowed: false,
			expectedCauseParts: []string{
				"Service CIDR \"10.96.0.0/25\" is too small",
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
			name:               "pod service overlap",
			podCIDRs:           []string{"10.96.0.0/16"},
			serviceCIDRs:       []string{"10.96.0.0/12"},
			expectAllowed:      false,
			expectedWarnings:   0,
			expectedCauseParts: []string{"overlaps with Service CIDR"},
		},
		{
			name:                 "pod and service overlap with node subnet",
			podCIDRs:             []string{"10.244.0.0/16"},
			serviceCIDRs:         []string{"10.96.0.0/12"},
			resolveSubnetCIDRs:   []netip.Prefix{netip.MustParsePrefix("10.244.128.0/24"), netip.MustParsePrefix("10.100.0.0/16")},
			withConfiguredSubnet: true,
			withNClient:          true,
			expectAllowed:        false,
			expectedWarnings:     0,
			expectedCauseParts: []string{
				"Pod CIDR \"10.244.0.0/16\" overlaps with node subnet CIDR \"10.244.128.0/24\"",
				"Service CIDR \"10.96.0.0/12\" overlaps with node subnet CIDR \"10.100.0.0/16\"",
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
			expectedCauseParts:   []string{"Failed to resolve node subnet CIDRs"},
		},
		{
			name:                 "missing nclient with configured subnets fails gracefully",
			podCIDRs:             []string{"10.244.0.0/16"},
			serviceCIDRs:         []string{"10.96.0.0/12"},
			withConfiguredSubnet: true,
			withNClient:          false,
			expectAllowed:        false,
			expectInternalError:  true,
			expectedWarnings:     0,
			expectedCauseParts:   []string{"Cannot validate subnet overlaps: Prism Central connection is not available"},
		},
		{
			name:               "invalid pod cidr",
			podCIDRs:           []string{"not-a-cidr"},
			serviceCIDRs:       []string{"10.96.0.0/12"},
			expectAllowed:      false,
			expectedCauseParts: []string{"Invalid Pod CIDR configuration"},
		},
		{
			name:               "invalid service cidr",
			podCIDRs:           []string{"10.244.0.0/16"},
			serviceCIDRs:       []string{"not-a-cidr"},
			expectAllowed:      false,
			expectedCauseParts: []string{"Invalid Service CIDR configuration"},
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
				resolveSubnetPrefixesFunc: func(
					ctx context.Context,
					nclient client,
					ids []capxv1.NutanixResourceIdentifier,
				) ([]netip.Prefix, error) {
					if !tt.withConfiguredSubnet {
						require.Empty(t, ids)
					} else {
						require.NotEmpty(t, ids)
					}
					if tt.resolveSubnetErr != nil {
						return nil, tt.resolveSubnetErr
					}
					return tt.resolveSubnetCIDRs, nil
				},
			}

			result := check.Run(context.Background())

			assert.Equal(t, tt.expectAllowed, result.Allowed, "Allowed mismatch")
			assert.Equal(t, tt.expectInternalError, result.InternalError, "InternalError mismatch")
			assert.Len(t, result.Warnings, tt.expectedWarnings, "Warnings count mismatch")

			for _, part := range tt.expectedCauseParts {
				assert.Contains(
					t,
					flattenCauseMessages(result.Causes),
					part,
					"expected cause to contain %q",
					part,
				)
			}
		})
	}
}

func TestCollectSubnetIdentifiers(t *testing.T) {
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
							subnetByName, // duplicate, should be deduped
						},
					},
				},
			},
		},
	}

	ids := collectSubnetIdentifiers(cd)
	assert.Len(t, ids, 2)
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

func flattenCauseMessages(causes []preflight.Cause) string {
	messages := make([]string, 0, len(causes))
	for _, cause := range causes {
		messages = append(messages, cause.Message)
	}
	return fmt.Sprintf("%v", messages)
}
