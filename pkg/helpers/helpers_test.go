// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsIPInRange(t *testing.T) {
	tests := []struct {
		name            string
		ipRange         string
		targetIP        string
		expectedInRange bool
		expectedErr     error
	}{
		{
			name:            "Valid range - target within range",
			ipRange:         "192.168.1.1-192.168.1.10",
			targetIP:        "192.168.1.5",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Valid range - target same as start IP",
			ipRange:         "192.168.1.1-192.168.1.10",
			targetIP:        "192.168.1.1",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Valid range - target same as end IP",
			ipRange:         "192.168.1.1-192.168.1.10",
			targetIP:        "192.168.1.10",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Valid range - target outside range",
			ipRange:         "192.168.1.1-192.168.1.10",
			targetIP:        "192.168.1.15",
			expectedInRange: false,
			expectedErr:     nil,
		},
		{
			name:            "Invalid start IP",
			ipRange:         "invalidIP-192.168.1.10",
			targetIP:        "192.168.1.5",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"failed to parse IP range %q: invalid From IP %q in range %q",
				"invalidIP-192.168.1.10",
				"invalidIP",
				"invalidIP-192.168.1.10",
			),
		},
		{
			name:            "Invalid end IP",
			ipRange:         "192.168.1.1-invalidIP",
			targetIP:        "192.168.1.5",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"failed to parse IP range %q: invalid To IP %q in range %q",
				"192.168.1.1-invalidIP",
				"invalidIP",
				"192.168.1.1-invalidIP",
			),
		},
		{
			name:            "Invalid target IP",
			ipRange:         "192.168.1.1-192.168.1.10",
			targetIP:        "invalidIP",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"failed to parse target IP %q: ParseAddr(%q): unable to parse IP",
				"invalidIP",
				"invalidIP",
			),
		},
		{
			name:            "IPv6 range - target within range",
			ipRange:         "2001:db8::1-2001:db8::10",
			targetIP:        "2001:db8::5",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "IPv6 range - target outside range",
			ipRange:         "2001:db8::1-2001:db8::10",
			targetIP:        "2001:db8::11",
			expectedInRange: false,
			expectedErr:     nil,
		},
		{
			name:            "IP prefix - target IP inside range",
			ipRange:         "192.168.1.1/25",
			targetIP:        "192.168.1.1",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "IP prefix - target IP outside range",
			ipRange:         "192.168.1.1/25",
			targetIP:        "192.168.1.251",
			expectedInRange: false,
			expectedErr:     nil,
		},
		{
			name:            "Invalid IP prefix",
			ipRange:         "192.168.1/25",
			targetIP:        "192.168.1.251",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"failed to parse IP prefix %q: netip.ParsePrefix(%q): ParseAddr(%q): IPv4 address too short",
				"192.168.1/25",
				"192.168.1/25",
				"192.168.1",
			),
		},
		{
			name:            "Single IP - same as target IP",
			ipRange:         "192.168.1.21",
			targetIP:        "192.168.1.21",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Single IP - different from target IP",
			ipRange:         "192.168.1.21",
			targetIP:        "192.168.1.211",
			expectedInRange: false,
			expectedErr:     nil,
		},
		{
			name:            "Invalid single IP",
			ipRange:         "192.168.1",
			targetIP:        "192.168.1.211",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"failed to parse IP address %q: ParseAddr(%q): IPv4 address too short",
				"192.168.1",
				"192.168.1",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsIPInRange(tt.ipRange, tt.targetIP)
			assert.Equal(t, tt.expectedInRange, got)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
