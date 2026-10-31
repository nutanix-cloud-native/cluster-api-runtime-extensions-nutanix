// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"net/netip"
	"testing"

	multidomainv4 "github.com/nutanix/ntnx-api-golang-clients/multidomain-go-client/v4/models/multidomain/v4/config"
	networkingcommonapi "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/common/v1/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	converged "github.com/nutanix-cloud-native/prism-go-client/converged"

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
					messagePart: "which is too small for a cluster",
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
					field:  "$.spec.topology.workers.machineDeployments[?@.name==\"md-0\"].variables[?@.name=='workerConfig'].value.nutanix.machineDetails.subnets[0]", //nolint:lll // Field path is long.
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
					field:       `$.spec.topology.workers.machineDeployments[?@.name=="md-0"].variables[?@.name=='workerConfig'].value.nutanix.machineDetails.subnets[0]`,
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
		{
			name:         "ambiguous subnet name error",
			podCIDRs:     []string{"10.244.0.0/16"},
			serviceCIDRs: []string{"10.96.0.0/12"},
			resolveSubnetErr: fmt.Errorf(
				"found 2 subnets matching name \"vlan170\" on the target Prism Element cluster; there must be exactly 1; use subnet UUID instead",
			),
			withConfiguredSubnet: true,
			withNClient:          true,
			expectAllowed:        false,
			expectInternalError:  true,
			expectedCauses: []expectedCause{
				{
					messagePart: "use subnet UUID instead",
					field:       "",
				},
			},
		},
		{
			name:             "empty pod CIDRs allowed",
			podCIDRs:         nil,
			serviceCIDRs:     []string{"10.96.0.0/12"},
			expectAllowed:    true,
			expectedWarnings: 0,
		},
		{
			name:             "empty service CIDRs allowed",
			podCIDRs:         []string{"10.244.0.0/16"},
			serviceCIDRs:     nil,
			expectAllowed:    true,
			expectedWarnings: 0,
		},
		{
			name:             "both pod and service CIDRs empty allowed",
			podCIDRs:         nil,
			serviceCIDRs:     nil,
			expectAllowed:    true,
			expectedWarnings: 0,
		},
		{
			name:          "empty pod CIDRs still validates service CIDR size",
			podCIDRs:      nil,
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
			name:          "empty service CIDRs still validates pod CIDR size",
			podCIDRs:      []string{"10.244.0.0/24"},
			serviceCIDRs:  nil,
			expectAllowed: false,
			expectedCauses: []expectedCause{
				{
					messagePart: "which is too small for a cluster",
					field:       "$.spec.clusterNetwork.pods.cidrBlocks",
				},
			},
			expectedWarnings: 0,
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
					_ *checkDependencies,
					_ client,
					subnets []nutanixSubnet,
				) ([]resolvedNodeSubnet, []string, error) {
					if !tt.withConfiguredSubnet {
						require.Empty(t, subnets)
					} else {
						require.NotEmpty(t, subnets)
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
		Name: new("subnet-a"),
	}
	subnetByUUID := capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("11111111-1111-1111-1111-111111111111"),
	}
	cpCluster := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new("cp-pe-uuid"),
	}
	workerCluster := &capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("worker-pe"),
	}

	cd := &checkDependencies{
		nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
			ControlPlane: &carenv1.NutanixControlPlaneSpec{
				Nutanix: &carenv1.NutanixControlPlaneNodeSpec{
					MachineDetails: carenv1.NutanixMachineDetails{
						Cluster: cpCluster,
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
						Cluster: workerCluster,
						Subnets: []capxv1.NutanixResourceIdentifier{
							subnetByName,
						},
					},
				},
			},
		},
	}

	subnets := collectNutanixSubnets(cd)
	// 2 from control plane + 1 from worker = 3 (no dedup, subnets are distinct)
	require.Len(t, subnets, 3)

	assert.Equal(t,
		`$.spec.topology.variables[?@.name=="clusterConfig"].value.controlPlane.nutanix.machineDetails.subnets[0]`,
		subnets[0].field,
	)
	assert.Equal(t, cpCluster, subnets[0].cluster)

	assert.Equal(t,
		`$.spec.topology.variables[?@.name=="clusterConfig"].value.controlPlane.nutanix.machineDetails.subnets[1]`,
		subnets[1].field,
	)
	assert.Equal(t, cpCluster, subnets[1].cluster)

	assert.Equal(
		t,
		`$.spec.topology.workers.machineDeployments[?@.name=="md-1"].variables[?@.name=='workerConfig'].value.nutanix.machineDetails.subnets[0]`,
		subnets[2].field,
	)
	assert.Equal(t, workerCluster, subnets[2].cluster)
}

func TestFilterSubnetsByPEClusterUsesClusterReferenceList(t *testing.T) {
	t.Parallel()

	subnets := []netv4.Subnet{
		{
			SubnetType:           ptr.To(netv4.SUBNETTYPE_VLAN),
			ClusterReferenceList: []string{"pe-1"},
		},
		{
			SubnetType:           ptr.To(netv4.SUBNETTYPE_VLAN),
			ClusterReferenceList: []string{"pe-2"},
		},
	}

	filtered, err := filterSubnetsByPECluster(
		context.Background(),
		nil,
		&clientWrapper{},
		subnets,
		&capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierUUID, UUID: new("pe-1")},
		nil,
	)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, []string{"pe-1"}, filtered[0].ClusterReferenceList)
}

func TestResolveNodeSubnetsDoesNotConflateSameNameAcrossClusters(t *testing.T) {
	t.Parallel()

	// Two distinct subnets share the name "shared-name" but live on different PE clusters.
	allSubnets := []netv4.Subnet{
		{
			Name:                 new("shared-name"),
			SubnetType:           ptr.To(netv4.SUBNETTYPE_VLAN),
			ClusterReferenceList: []string{"pe-1"},
			IpConfig: []netv4.IPConfig{{
				Ipv4: &netv4.IPv4Config{
					IpSubnet: &netv4.IPv4Subnet{
						Ip:           &networkingcommonapi.IPv4Address{Value: new("10.1.0.0")},
						PrefixLength: new(24),
					},
				},
			}},
		},
		{
			Name:                 new("shared-name"),
			SubnetType:           ptr.To(netv4.SUBNETTYPE_VLAN),
			ClusterReferenceList: []string{"pe-2"},
			IpConfig: []netv4.IPConfig{{
				Ipv4: &netv4.IPv4Config{
					IpSubnet: &netv4.IPv4Subnet{
						Ip:           &networkingcommonapi.IPv4Address{Value: new("10.2.0.0")},
						PrefixLength: new(24),
					},
				},
			}},
		},
	}

	nclient := &clientWrapper{
		ListSubnetsByFilterFunc: func(context.Context, string) ([]netv4.Subnet, error) {
			return allSubnets, nil
		},
	}

	subnets := []nutanixSubnet{
		{
			id:      capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierName, Name: new("shared-name")},
			field:   "controlPlane",
			cluster: &capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierUUID, UUID: new("pe-1")},
		},
		{
			id:      capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierName, Name: new("shared-name")},
			field:   "worker",
			cluster: &capxv1.NutanixResourceIdentifier{Type: capxv1.NutanixIdentifierUUID, UUID: new("pe-2")},
		},
	}

	resolved, warnings, err := resolveNodeSubnets(context.Background(), nil, nclient, subnets)
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, resolved, 2)

	byField := map[string]netip.Prefix{}
	for _, r := range resolved {
		byField[r.field] = r.prefix
	}
	assert.Equal(t, netip.MustParsePrefix("10.1.0.0/24"), byField["controlPlane"])
	assert.Equal(t, netip.MustParsePrefix("10.2.0.0/24"), byField["worker"])
}

func TestResolvePEClusterUUIDProjectScoped(t *testing.T) {
	t.Parallel()

	cd := &checkDependencies{pcVersion: "7.6"}
	projectRef := new(capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierUUID,
		UUID: new(subnetTestProjectUUID),
	})
	clusterRef := capxv1.NutanixResourceIdentifier{
		Type: capxv1.NutanixIdentifierName,
		Name: new("pe-project-scoped"),
	}

	t.Run("resolved via project resource groups, never calls getClusters", func(t *testing.T) {
		t.Parallel()

		nclient := &clientWrapper{
			ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
				return nil, projectScopedDefaultProjectErr()
			},
			ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
				return &resolvedProject{UUID: subnetTestProjectUUID}, nil
			},
			ListResourceGroupsFunc: func(
				context.Context, ...converged.ODataOption,
			) ([]multidomainv4.ResourceGroup, error) {
				return []multidomainv4.ResourceGroup{{ExtId: new("rg-1"), Name: new("rg-1")}}, nil
			},
			ListPrismElementsByResourceGroupFunc: func(
				_ context.Context, _ string,
			) ([]converged.PrismElementInfo, error) {
				return []converged.PrismElementInfo{{ExtId: "pe-uuid-from-project", Name: "pe-project-scoped"}}, nil
			},
			// No ListClustersFunc/GetClusterByIdFunc set: a call to the global cluster-management
			// API would fail with "Func is not set", so success here proves the project-scoped
			// path was taken instead.
		}

		peUUID, err := resolvePEClusterUUID(context.Background(), cd, nclient, &clusterRef, projectRef)
		require.NoError(t, err)
		assert.Equal(t, "pe-uuid-from-project", peUUID)
	})

	t.Run("PE not available in project scope - error", func(t *testing.T) {
		t.Parallel()

		nclient := &clientWrapper{
			ResolvePCDefaultProjectFunc: func(context.Context, string) (*PCDefaultProject, error) {
				return nil, projectScopedDefaultProjectErr()
			},
			ResolveProjectFunc: func(context.Context, *capxv1.NutanixResourceIdentifier, string) (*resolvedProject, error) {
				return &resolvedProject{UUID: subnetTestProjectUUID}, nil
			},
			ListResourceGroupsFunc: func(
				context.Context, ...converged.ODataOption,
			) ([]multidomainv4.ResourceGroup, error) {
				return []multidomainv4.ResourceGroup{{ExtId: new("rg-1"), Name: new("rg-1")}}, nil
			},
			ListPrismElementsByResourceGroupFunc: func(
				_ context.Context, _ string,
			) ([]converged.PrismElementInfo, error) {
				return []converged.PrismElementInfo{{ExtId: "some-other-pe-uuid", Name: "some-other-pe"}}, nil
			},
		}

		_, err := resolvePEClusterUUID(context.Background(), cd, nclient, &clusterRef, projectRef)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not available in the selected project scope")
	})
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
									Value: new("10.0.0.0"),
								},
								PrefixLength: new(24),
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

func testCluster(podCIDRs, serviceCIDRs []string) *clusterv1beta2.Cluster {
	return &clusterv1beta2.Cluster{
		Spec: clusterv1beta2.ClusterSpec{
			ClusterNetwork: clusterv1beta2.ClusterNetwork{
				Pods: clusterv1beta2.NetworkRanges{
					CIDRBlocks: podCIDRs,
				},
				Services: clusterv1beta2.NetworkRanges{
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
			Name: new("subnet-a"),
		},
	}
}
