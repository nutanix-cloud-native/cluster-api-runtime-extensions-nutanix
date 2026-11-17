// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"errors"
	"testing"

	clustermgmtv4 "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	netv4 "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

const autoIPAMSubnetUUID = "d4ade013-d1d1-4eac-9ab8-cdcc91ad4cec"

func subnetsResponse(t *testing.T, subnets []netv4.Subnet) *netv4.ListSubnetsApiResponse {
	t.Helper()
	resp := &netv4.ListSubnetsApiResponse{}
	resp.Data = netv4.NewOneOfListSubnetsApiResponseData()
	require.NoError(t, resp.Data.SetValue(subnets))
	return resp
}

func clustersResponse(t *testing.T, clusters []clustermgmtv4.Cluster) *clustermgmtv4.ListClustersApiResponse {
	t.Helper()
	resp := &clustermgmtv4.ListClustersApiResponse{}
	resp.Data = clustermgmtv4.NewOneOfListClustersApiResponseData()
	require.NoError(t, resp.Data.SetValue(clusters))
	return resp
}

func namedCluster(name, extID string) clustermgmtv4.Cluster {
	return clustermgmtv4.Cluster{Name: ptr.To(name), ExtId: ptr.To(extID)}
}

func namedSubnet(name, extID, clusterRef string) netv4.Subnet {
	s := netv4.Subnet{Name: ptr.To(name), ExtId: ptr.To(extID)}
	if clusterRef != "" {
		s.ClusterReference = ptr.To(clusterRef)
	}
	return s
}

// A UUID goes straight to the reservation probe: no name resolution is needed
// and a subnet that does not exist is reported by the probe.
func TestAutoIPAMCheckSubnetUUID(t *testing.T) {
	t.Parallel()

	var probed string
	check := &serviceLoadBalancerAutoIPAMCheck{
		name: "NutanixServiceLoadBalancerAutoIPAM", field: autoIPAMField,
		purpose: "Service LoadBalancer", component: "Nutanix CCM",
		subnet: autoIPAMSubnetUUID,
		nclient: &clientWrapper{
			ListReservedIpsBySubnetIdFunc: func(_ context.Context, subnetExtID string, limit *int) error {
				probed = subnetExtID
				assert.Equal(t, 1, *limit, "the probe must not page the whole subnet")
				return nil
			},
		},
	}

	result := check.Run(context.Background())
	assert.True(t, result.Allowed, result.Causes)
	assert.Equal(t, autoIPAMSubnetUUID, probed)
}

// A name is resolved first, narrowed by the Prism Element cluster, and the
// resolved extID is what gets probed.
func TestAutoIPAMCheckSubnetName(t *testing.T) {
	t.Parallel()

	// A subnet reports its cluster as a UUID, so these are the extIDs the
	// Prism Element names resolve to.
	const (
		peAUUID = "00065e57-0f1b-17bc-289f-7cc25586eef1"
		peBUUID = "fc3b713a-e5cc-469c-954e-25ed36c7ddcd"
	)
	clusters := []clustermgmtv4.Cluster{
		namedCluster("pe-a", peAUUID),
		namedCluster("pe-b", peBUUID),
	}

	tests := []struct {
		name           string
		subnetName     string
		peCluster      string
		subnets        []netv4.Subnet
		filterErr      error
		wantProbed     string
		wantAllowed    bool
		wantCause      string
		wantUnfiltered bool
	}{
		{
			name:        "a unique name resolves",
			subnetName:  "nkp_managed",
			subnets:     []netv4.Subnet{namedSubnet("nkp_managed", autoIPAMSubnetUUID, peAUUID)},
			wantProbed:  autoIPAMSubnetUUID,
			wantAllowed: true,
		},
		{
			// The CLI passes the Prism Element name the operator typed, so the
			// name has to be resolved before it can narrow anything. Comparing
			// the name against the subnet's clusterReference never matches.
			name:        "a Prism Element given by name narrows a duplicated subnet name",
			subnetName:  "shared",
			peCluster:   "pe-b",
			subnets:     []netv4.Subnet{namedSubnet("shared", "s-a", peAUUID), namedSubnet("shared", "s-b", peBUUID)},
			wantProbed:  "s-b",
			wantAllowed: true,
		},
		{
			name:        "a Prism Element given by UUID narrows too",
			subnetName:  "shared",
			peCluster:   peBUUID,
			subnets:     []netv4.Subnet{namedSubnet("shared", "s-a", peAUUID), namedSubnet("shared", "s-b", peBUUID)},
			wantProbed:  "s-b",
			wantAllowed: true,
		},
		{
			name:        "an unknown Prism Element name is rejected",
			subnetName:  "shared",
			peCluster:   "pe-does-not-exist",
			subnets:     []netv4.Subnet{namedSubnet("shared", "s-a", peAUUID)},
			wantAllowed: false,
			wantCause:   `no Prism Element cluster named "pe-does-not-exist"`,
		},
		{
			// Prism Central rejects a filter holding non-ASCII characters, so the
			// name must still resolve from a full listing. Without this the check
			// would block a cluster the CCM handles fine.
			name:       "a non-ascii name resolves without the server filter",
			subnetName: "aßa",
			subnets: []netv4.Subnet{
				namedSubnet("aẞa", "s-other", peAUUID),
				namedSubnet("aßa", "s-sharp", peAUUID),
			},
			wantProbed:     "s-sharp",
			wantAllowed:    true,
			wantUnfiltered: true,
		},
		{
			// A rejected filter must degrade to a full listing, not fail.
			name:           "a rejected filter falls back to a full listing",
			subnetName:     "nkp_managed",
			filterErr:      errors.New("NETWORKING-10002: Odata Error: Error while parsing URI"),
			subnets:        []netv4.Subnet{namedSubnet("nkp_managed", autoIPAMSubnetUUID, peAUUID)},
			wantProbed:     autoIPAMSubnetUUID,
			wantAllowed:    true,
			wantUnfiltered: true,
		},
		{
			name:        "a duplicated name with no cluster is rejected, not guessed",
			subnetName:  "shared",
			subnets:     []netv4.Subnet{namedSubnet("shared", "s-a", peAUUID), namedSubnet("shared", "s-b", peBUUID)},
			wantAllowed: false,
			wantCause:   "matches 2 subnets",
		},
		{
			name:        "an absent name is rejected",
			subnetName:  "does-not-exist",
			subnets:     []netv4.Subnet{namedSubnet("nkp_managed", autoIPAMSubnetUUID, peAUUID)},
			wantAllowed: false,
			wantCause:   `no subnet named "does-not-exist"`,
		},
		{
			// A server that answered with the wrong row must not be believed.
			name:        "a name the server mismatched is not accepted",
			subnetName:  "aßa",
			subnets:     []netv4.Subnet{namedSubnet("aẞa", "s-other", peAUUID)},
			wantAllowed: false,
			wantCause:   "no subnet named",
		},
		{
			// An overlay subnet declares no cluster, and the cluster is filled in
			// automatically whenever a name is used.
			name:       "a subnet with no cluster is not narrowed away",
			subnetName: "overlay",
			peCluster:  "pe-a",
			//nolint:lll // The comment is above.
			subnets:     []netv4.Subnet{namedSubnet("overlay", "s-overlay", "")},
			wantProbed:  "s-overlay",
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var probed string
			var usedFilter, usedUnfiltered bool
			check := &serviceLoadBalancerAutoIPAMCheck{
				name: "NutanixServiceLoadBalancerAutoIPAM", field: autoIPAMField,
				purpose: "Service LoadBalancer", component: "Nutanix CCM",
				subnet:              tt.subnetName,
				prismElementCluster: tt.peCluster,
				nclient: &clientWrapper{
					ListClustersFunc: func(
						context.Context, *int, *int, *string, *string, *string, *string, ...map[string]any,
					) (*clustermgmtv4.ListClustersApiResponse, error) {
						return clustersResponse(t, clusters), nil
					},
					ListSubnetsFunc: func(
						_ context.Context, _, _ *int, filter, _, _, _ *string, _ ...map[string]any,
					) (*netv4.ListSubnetsApiResponse, error) {
						if filter != nil {
							usedFilter = true
							if tt.filterErr != nil {
								return nil, tt.filterErr
							}
						} else {
							usedUnfiltered = true
						}
						return subnetsResponse(t, tt.subnets), nil
					},
					ListReservedIpsBySubnetIdFunc: func(_ context.Context, subnetExtID string, _ *int) error {
						probed = subnetExtID
						return nil
					},
				},
			}

			result := check.Run(context.Background())
			assert.Equal(t, tt.wantAllowed, result.Allowed, result.Causes)
			assert.Equal(t, tt.wantProbed, probed)
			if tt.wantCause != "" {
				require.NotEmpty(t, result.Causes)
				assert.Contains(t, result.Causes[0].Message, tt.wantCause)
			}
			if tt.wantUnfiltered {
				assert.True(t, usedUnfiltered, "the full listing fallback was not used")
			}
			_ = usedFilter
		})
	}
}

// These are the refusals Prism Central actually gives, and each has to produce
// a different message: the operator's next action is different for each.
func TestAutoIPAMCheckProbeFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		probeErr          error
		wantInternalError bool
		wantCause         string
	}{
		{
			// Measured on PC 7.6 GA against a subnet with no IP pool. Without
			// this check the cluster comes up and every LoadBalancer Service
			// then hangs in Pending.
			name: "an unmanaged subnet can never be reserved from",
			probeErr: errors.New(
				"NETWORKING-10056: Application error kInvalidArgument raised: " +
					"Cannot fetch reserved IP addresses for unmanaged subnets.",
			),
			wantCause: "is not IP address managed",
		},
		{
			// Measured on PC 7.6 GA for a subnet UUID that does not exist. This
			// must read as a configuration error: calling it temporary would have
			// the operator retry a cluster that can never be created.
			name: "a subnet that does not exist is a configuration error",
			probeErr: errors.New(
				"NETWORKING-10060: Application error kNotFound raised: " +
					"Existing subnet: 00000000-0000-0000-0000-000000000000 is not found.",
			),
			wantCause: "does not exist on this Prism Central",
		},
		{
			name:      "a refused read is a privilege problem",
			probeErr:  errors.New("403 Forbidden"),
			wantCause: "not permitted to read IP reservations",
		},
		{
			name:      "an unauthenticated read is a privilege problem",
			probeErr:  errors.New("401 Unauthorized"),
			wantCause: "not permitted to read IP reservations",
		},
		{
			// Anything else may be transient, so it must not read as a
			// misconfiguration the operator has to fix.
			name:              "an unreachable Prism Central is retryable",
			probeErr:          errors.New("connection refused"),
			wantInternalError: true,
			wantCause:         "temporary error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			check := &serviceLoadBalancerAutoIPAMCheck{
				name: "NutanixServiceLoadBalancerAutoIPAM", field: autoIPAMField,
				purpose: "Service LoadBalancer", component: "Nutanix CCM",
				subnet: autoIPAMSubnetUUID,
				nclient: &clientWrapper{
					ListReservedIpsBySubnetIdFunc: func(context.Context, string, *int) error {
						return tt.probeErr
					},
				},
			}

			result := check.Run(context.Background())
			assert.False(t, result.Allowed)
			assert.Equal(t, tt.wantInternalError, result.InternalError)
			require.NotEmpty(t, result.Causes)
			assert.Contains(t, result.Causes[0].Message, tt.wantCause)
			assert.Contains(t, result.Causes[0].Field, "autoIPAM.subnet")
		})
	}
}

func TestAutoIPAMCheckEmptySubnet(t *testing.T) {
	t.Parallel()

	check := &serviceLoadBalancerAutoIPAMCheck{
		name: "NutanixServiceLoadBalancerAutoIPAM", field: autoIPAMField,
		purpose: "Service LoadBalancer", component: "Nutanix CCM",
		subnet: "",
		nclient: &clientWrapper{
			ListReservedIpsBySubnetIdFunc: func(context.Context, string, *int) error {
				t.Fatal("Prism Central was called with no subnet configured")
				return nil
			},
		},
	}

	result := check.Run(context.Background())
	assert.False(t, result.Allowed)
	require.NotEmpty(t, result.Causes)
	assert.Contains(t, result.Causes[0].Message, "requires a subnet")
}

// The check exists only for automatic mode. A static-range cluster, or one with
// no Service LoadBalancer, must not make a Prism Central call at all.
func TestNewServiceLoadBalancerAutoIPAMChecks(t *testing.T) {
	t.Parallel()

	autoIPAM := &carenv1.ServiceLoadBalancerAutoIPAM{Subnet: autoIPAMSubnetUUID, Cluster: ptr.To("pe-a")}

	tests := []struct {
		name      string
		cd        *checkDependencies
		wantCount int
	}{
		{
			name:      "no cluster config",
			cd:        &checkDependencies{nclient: &clientWrapper{}},
			wantCount: 0,
		},
		{
			name: "no client yet",
			cd: &checkDependencies{
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					Addons: &carenv1.NutanixAddons{
						GenericAddons: carenv1.GenericAddons{
							ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
								Provider:      carenv1.ServiceLoadBalancerProviderMetalLB,
								Configuration: &carenv1.ServiceLoadBalancerConfiguration{AutoIPAM: autoIPAM},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "no addons",
			cd: &checkDependencies{
				nclient:                  &clientWrapper{},
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{},
			},
			wantCount: 0,
		},
		{
			name: "static address ranges",
			cd: &checkDependencies{
				nclient: &clientWrapper{},
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					Addons: &carenv1.NutanixAddons{
						GenericAddons: carenv1.GenericAddons{
							ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
								Provider: carenv1.ServiceLoadBalancerProviderMetalLB,
								Configuration: &carenv1.ServiceLoadBalancerConfiguration{
									AddressRanges: []carenv1.AddressRange{{Start: "10.0.0.10", End: "10.0.0.20"}},
								},
							},
						},
					},
				},
			},
			wantCount: 0,
		},
		{
			name: "automatic mode",
			cd: &checkDependencies{
				nclient: &clientWrapper{},
				nutanixClusterConfigSpec: &carenv1.NutanixClusterConfigSpec{
					Addons: &carenv1.NutanixAddons{
						GenericAddons: carenv1.GenericAddons{
							ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
								Provider:      carenv1.ServiceLoadBalancerProviderMetalLB,
								Configuration: &carenv1.ServiceLoadBalancerConfiguration{AutoIPAM: autoIPAM},
							},
						},
					},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checks := newServiceLoadBalancerAutoIPAMChecks(tt.cd)
			assert.Len(t, checks, tt.wantCount)
			if tt.wantCount == 1 {
				assert.Equal(t, "NutanixServiceLoadBalancerAutoIPAM", checks[0].Name())
				autoCheck, ok := checks[0].(*serviceLoadBalancerAutoIPAMCheck)
				require.True(t, ok)
				assert.Equal(t, "pe-a", autoCheck.prismElementCluster)
			}
		})
	}
}

var _ preflight.Check = &serviceLoadBalancerAutoIPAMCheck{}

// The gap this closes: only the Service LoadBalancer subnet was preflighted, so
// an unmanaged or absent subnet on the API server reservation produced no
// failure at create time. The cluster was accepted and then waited forever,
// with the real cause ("no IP prefix found for subnet") only in the IPAM
// provider's log. Measured against a live unmanaged subnet.
func TestReservationChecksCoverBothSubnets(t *testing.T) {
	t.Parallel()

	autoIPAM := &carenv1.ServiceLoadBalancerAutoIPAM{Subnet: "lb-subnet"}
	reservation := &carenv1.ControlPlaneIPReservation{Subnet: "cp-subnet"}

	names := func(cfg *carenv1.NutanixClusterConfigSpec) []string {
		checks := newServiceLoadBalancerAutoIPAMChecks(&checkDependencies{
			nutanixClusterConfigSpec: cfg,
			nclient:                  &clientWrapper{},
		})
		out := make([]string, 0, len(checks))
		for _, c := range checks {
			out = append(out, c.Name())
		}
		return out
	}

	withCP := func() *carenv1.NutanixClusterConfigSpec {
		return &carenv1.NutanixClusterConfigSpec{
			Nutanix: &carenv1.NutanixSpec{
				ControlPlaneEndpoint: carenv1.ControlPlaneEndpointSpec{
					Port: 6443,
					VirtualIPSpec: &carenv1.ControlPlaneVirtualIPSpec{
						Configuration: &carenv1.ControlPlaneVirtualIPConfiguration{
							IPReservation: reservation,
						},
					},
				},
			},
		}
	}

	t.Run("the API server reservation alone is checked", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, []string{"NutanixControlPlaneEndpointIPReservation"}, names(withCP()))
	})

	t.Run("both reservations are checked", func(t *testing.T) {
		t.Parallel()
		cfg := withCP()
		cfg.Addons = &carenv1.NutanixAddons{
			GenericAddons: carenv1.GenericAddons{
				ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
					Provider:      carenv1.ServiceLoadBalancerProviderMetalLB,
					Configuration: &carenv1.ServiceLoadBalancerConfiguration{AutoIPAM: autoIPAM},
				},
			},
		}
		assert.Equal(t, []string{
			"NutanixServiceLoadBalancerAutoIPAM",
			"NutanixControlPlaneEndpointIPReservation",
		}, names(cfg))
	})

	t.Run("a cluster reserving nothing is checked for nothing", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, names(&carenv1.NutanixClusterConfigSpec{Nutanix: &carenv1.NutanixSpec{}}))
	})

	// The message has to name the right component, or it sends the operator to
	// the wrong place.
	t.Run("an unmanaged subnet blames the IPAM provider, not the CCM", func(t *testing.T) {
		t.Parallel()
		check := &serviceLoadBalancerAutoIPAMCheck{
			name: "NutanixControlPlaneEndpointIPReservation", field: ipReservationField,
			purpose: "API server address", component: "Nutanix IPAM provider",
			subnet: autoIPAMSubnetUUID,
			nclient: &clientWrapper{
				ListReservedIpsBySubnetIdFunc: func(context.Context, string, *int) error {
					return errors.New("NETWORKING-10056 Cannot fetch reserved IP addresses for unmanaged subnets")
				},
			},
		}
		result := check.Run(context.Background())
		require.False(t, result.Allowed)
		require.Len(t, result.Causes, 1)
		assert.Contains(t, result.Causes[0].Message, "not IP address managed")
		assert.Contains(t, result.Causes[0].Message, "Nutanix IPAM provider")
		assert.Contains(t, result.Causes[0].Field, "ipReservation")
	})
}
