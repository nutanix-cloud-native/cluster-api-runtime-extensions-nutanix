// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
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
			expectedErr: fmt.Errorf(
				"error parsing Prism Central URL: parse %q: invalid URI for request",
				"invalid-url",
			),
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

func TestCheckIfPrismCentralAndControlPlaneIPSame(t *testing.T) {
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
				"prism central and control plane endpoint cannot have the same IP %q",
				"192.168.1.1",
			),
		},
		{
			name: "Invalid Control Plane IP",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "invalid-ip",
			},
			expectedErr: fmt.Errorf(
				"invalid Nutanix control plane endpoint IP: ParseAddr(%q): unable to parse IP",
				"invalid-ip",
			),
		},
		{
			name: "Invalid Prism Central URL",
			pcEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "invalid-url",
			},
			controlPlaneEndpointSpec: v1alpha1.ControlPlaneEndpointSpec{
				Host: "192.168.1.2",
			},
			expectedErr: fmt.Errorf(
				"error parsing Prism Central URL: parse %q: invalid URI for request",
				"invalid-url",
			),
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
				"prism central and control plane endpoint cannot have the same IP %q",
				"192.168.1.1",
			),
		},
		{
			name: "With KubeVIP ovveride and different PC and Control Plane IPs",
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
			err := checkIfPrismCentralAndControlPlaneIPSame(
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
