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
		startIP         string
		endIP           string
		targetIP        string
		expectedInRange bool
		expectedErr     error
	}{
		{
			name:            "Valid range - target within range",
			startIP:         "192.168.1.1",
			endIP:           "192.168.1.10",
			targetIP:        "192.168.1.5",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Valid range - target same as start IP",
			startIP:         "192.168.1.1",
			endIP:           "192.168.1.10",
			targetIP:        "192.168.1.1",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Valid range - target same as end IP",
			startIP:         "192.168.1.1",
			endIP:           "192.168.1.10",
			targetIP:        "192.168.1.10",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "Valid range - target outside range",
			startIP:         "192.168.1.1",
			endIP:           "192.168.1.10",
			targetIP:        "192.168.1.15",
			expectedInRange: false,
			expectedErr:     nil,
		},
		{
			name:            "Invalid start IP",
			startIP:         "invalid-ip",
			endIP:           "192.168.1.10",
			targetIP:        "192.168.1.5",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"invalid start IP: ParseAddr(%q): unable to parse IP",
				"invalid-ip",
			),
		},
		{
			name:            "Invalid end IP",
			startIP:         "192.168.1.1",
			endIP:           "invalid-ip",
			targetIP:        "192.168.1.5",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"invalid end IP: ParseAddr(%q): unable to parse IP",
				"invalid-ip",
			),
		},
		{
			name:            "Invalid target IP",
			startIP:         "192.168.1.1",
			endIP:           "192.168.1.10",
			targetIP:        "invalid-ip",
			expectedInRange: false,
			expectedErr: fmt.Errorf(
				"invalid target IP: ParseAddr(%q): unable to parse IP",
				"invalid-ip",
			),
		},
		{
			name:            "IPv6 range - target within range",
			startIP:         "2001:db8::1",
			endIP:           "2001:db8::10",
			targetIP:        "2001:db8::5",
			expectedInRange: true,
			expectedErr:     nil,
		},
		{
			name:            "IPv6 range - target outside range",
			startIP:         "2001:db8::1",
			endIP:           "2001:db8::10",
			targetIP:        "2001:db8::11",
			expectedInRange: false,
			expectedErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsIPInRange(tt.startIP, tt.endIP, tt.targetIP)
			assert.Equal(t, tt.expectedInRange, got)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
