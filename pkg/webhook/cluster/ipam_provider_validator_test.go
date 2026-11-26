// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

// autoIPAMConfig is the shape of a cluster asking for its Service addresses to
// be reserved.
func autoIPAMConfig() *variables.ClusterConfigSpec {
	return &variables.ClusterConfigSpec{
		Addons: &variables.Addons{
			GenericAddons: carenv1.GenericAddons{
				ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
					Provider: carenv1.ServiceLoadBalancerProviderMetalLB,
					Configuration: &carenv1.ServiceLoadBalancerConfiguration{
						AutoIPAM: &carenv1.ServiceLoadBalancerAutoIPAM{Subnet: "vlan170"},
					},
				},
			},
		},
	}
}

// ipReservationConfig is the shape of a cluster asking for its API server
// address to be reserved.
func ipReservationConfig() *variables.ClusterConfigSpec {
	return &variables.ClusterConfigSpec{
		Nutanix: &carenv1.NutanixSpec{
			ControlPlaneEndpoint: carenv1.ControlPlaneEndpointSpec{
				Port: 6443,
				VirtualIPSpec: &carenv1.ControlPlaneVirtualIPSpec{
					Configuration: &carenv1.ControlPlaneVirtualIPConfiguration{
						IPReservation: &carenv1.ControlPlaneIPReservation{Subnet: "vlan170"},
					},
				},
			},
		},
	}
}

func staticRangesConfig() *variables.ClusterConfigSpec {
	return &variables.ClusterConfigSpec{
		Addons: &variables.Addons{
			GenericAddons: carenv1.GenericAddons{
				ServiceLoadBalancer: &carenv1.ServiceLoadBalancer{
					Provider: carenv1.ServiceLoadBalancerProviderMetalLB,
					Configuration: &carenv1.ServiceLoadBalancerConfiguration{
						AddressRanges: []carenv1.AddressRange{{Start: "10.0.0.10", End: "10.0.0.20"}},
					},
				},
			},
		},
	}
}

func TestUnsupportedReservationOnProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		provider     string
		config       *variables.ClusterConfigSpec
		wantRefused  bool
		wantMentions []string
	}{
		{
			name:     "nutanix may reserve Service addresses",
			provider: "nutanix",
			config:   autoIPAMConfig(),
		},
		{
			name:     "nutanix may reserve the API server address",
			provider: "nutanix",
			config:   ipReservationConfig(),
		},
		{
			// Nothing outside Nutanix implements this: the AWS CCM ignores the
			// setting, so the cluster would come up and its Services would
			// never get an address.
			name:         "aws may not reserve Service addresses",
			provider:     "aws",
			config:       autoIPAMConfig(),
			wantRefused:  true,
			wantMentions: []string{"only supported on Nutanix", "addressRanges", `"aws"`, "Nutanix CCM"},
		},
		{
			name:         "eks may not reserve Service addresses",
			provider:     "eks",
			config:       autoIPAMConfig(),
			wantRefused:  true,
			wantMentions: []string{`"eks"`},
		},
		{
			name:         "docker may not reserve Service addresses",
			provider:     "docker",
			config:       autoIPAMConfig(),
			wantRefused:  true,
			wantMentions: []string{`"docker"`},
		},
		{
			name:         "a non-nutanix cluster may not reserve the API server address",
			provider:     "aws",
			config:       ipReservationConfig(),
			wantRefused:  true,
			wantMentions: []string{"Nutanix IPAM provider", "set a host instead"},
		},
		{
			name:     "static ranges are fine anywhere",
			provider: "aws",
			config:   staticRangesConfig(),
		},
		{
			name:     "a cluster with no addons is fine",
			provider: "aws",
			config:   &variables.ClusterConfigSpec{},
		},
		{
			name:     "a nil config is fine",
			provider: "aws",
			config:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := unsupportedReservationOnProvider(tt.provider, tt.config)
			if !tt.wantRefused {
				assert.Empty(t, msg)
				return
			}
			assert.NotEmpty(t, msg, "this configuration silently does nothing and must be refused")
			for _, want := range tt.wantMentions {
				assert.Contains(t, msg, want)
			}
		})
	}
}

// The regression this guards, found by creating a real cluster: the provider
// label is applied after admission, so on create it is absent. Classifying from
// the label alone made a valid Nutanix cluster look like provider "" and the
// create was refused outright.
func TestClusterProviderIsKnownBeforeTheLabelExists(t *testing.T) {
	t.Parallel()

	unlabelled := &clusterv1.Cluster{} // exactly what admission sees on create

	nutanix := ipReservationConfig()
	assert.Equal(t, "nutanix", clusterProvider(unlabelled, nutanix),
		"a nutanix section identifies the provider with no label present")
	assert.Empty(t, unsupportedReservationOnProvider(clusterProvider(unlabelled, nutanix), nutanix),
		"a valid Nutanix cluster must not be refused for lacking a label it cannot have yet")

	aws := autoIPAMConfig()
	aws.AWS = &carenv1.AWSSpec{}
	assert.Equal(t, "aws", clusterProvider(unlabelled, aws))
	assert.NotEmpty(t, unsupportedReservationOnProvider(clusterProvider(unlabelled, aws), aws),
		"an AWS cluster is still caught at create time")
}

// A cluster whose provider cannot be told must be left alone rather than
// refused: blocking a valid cluster is worse than missing a misconfiguration
// that the reservation itself then fails on.
func TestUnclassifiableProviderIsAllowed(t *testing.T) {
	t.Parallel()

	cfg := autoIPAMConfig() // autoIPAM set, but no provider section at all
	assert.Empty(t, clusterProvider(&clusterv1.Cluster{}, cfg))
	assert.Empty(t, unsupportedReservationOnProvider("", cfg))
}

// Naming the API server address and reserving one are alternatives. kube-vip
// binds the virtual IP configuration's address and falls back to the endpoint
// host, so accepting both would silently ignore the reservation and hand
// kube-vip a host it may not be able to bind at all.
func TestConflictingEndpointSources(t *testing.T) {
	t.Parallel()

	externalWithReservation := func() *variables.ClusterConfigSpec {
		cfg := ipReservationConfig()
		cfg.Nutanix.ControlPlaneEndpoint.Host = "api.example.com"
		return cfg
	}
	namedAddressOnly := func() *variables.ClusterConfigSpec {
		return &variables.ClusterConfigSpec{
			Nutanix: &carenv1.NutanixSpec{
				ControlPlaneEndpoint: carenv1.ControlPlaneEndpointSpec{
					Host: "10.0.0.5",
					Port: 6443,
					VirtualIPSpec: &carenv1.ControlPlaneVirtualIPSpec{
						Provider: carenv1.VirtualIPProviderKubeVIP,
						Configuration: &carenv1.ControlPlaneVirtualIPConfiguration{
							Address: "10.0.0.5",
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name     string
		config   *variables.ClusterConfigSpec
		refused  bool
		contains string
	}{
		{
			name:     "an external endpoint with a reservation is refused",
			config:   externalWithReservation(),
			refused:  true,
			contains: "api.example.com",
		},
		{
			name:    "a reservation on its own is allowed",
			config:  ipReservationConfig(),
			refused: false,
		},
		{
			name:    "an external endpoint fronting a named address is allowed",
			config:  namedAddressOnly(),
			refused: false,
		},
		{
			name:    "a cluster with no nutanix section is allowed",
			config:  autoIPAMConfig(),
			refused: false,
		},
		{
			name:    "a nil config is allowed",
			config:  nil,
			refused: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := conflictingEndpointSources(tt.config)
			if !tt.refused {
				assert.Empty(t, msg)
				return
			}
			assert.NotEmpty(t, msg)
			assert.Contains(t, msg, tt.contains)
			assert.Contains(t, msg, "not both")
		})
	}
}
