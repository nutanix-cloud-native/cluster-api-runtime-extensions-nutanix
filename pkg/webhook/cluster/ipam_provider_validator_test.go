// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
