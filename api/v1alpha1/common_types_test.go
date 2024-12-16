// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestControlPlaneEndpointIP(t *testing.T) {
	tests := []struct {
		name     string
		spec     ControlPlaneEndpointSpec
		expected string
	}{
		{
			name: "Virtual IP specified",
			spec: ControlPlaneEndpointSpec{
				VirtualIPSpec: &ControlPlaneVirtualIPSpec{
					Configuration: &ControlPlaneVirtualIPConfiguration{
						Address: "192.168.1.1",
					},
				},
				Host: "192.168.1.2",
			},
			expected: "192.168.1.1",
		},
		{
			name: "VirtualIPSpec struct not specified",
			spec: ControlPlaneEndpointSpec{
				VirtualIPSpec: nil,
				Host:          "192.168.1.2",
			},
			expected: "192.168.1.2",
		},
		{
			name: "ControlPlaneVirtualIPConfiguration struct not specified",
			spec: ControlPlaneEndpointSpec{
				VirtualIPSpec: &ControlPlaneVirtualIPSpec{
					Configuration: nil,
				},
				Host: "192.168.1.2",
			},
			expected: "192.168.1.2",
		},
		{
			name: "Virtual IP specified as empty string",
			spec: ControlPlaneEndpointSpec{
				VirtualIPSpec: &ControlPlaneVirtualIPSpec{
					Configuration: &ControlPlaneVirtualIPConfiguration{
						Address: "",
					},
				},
				Host: "192.168.1.2",
			},
			expected: "192.168.1.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.spec.VirtualIPAddress()
			assert.Equal(t, tt.expected, result)
		})
	}
}
