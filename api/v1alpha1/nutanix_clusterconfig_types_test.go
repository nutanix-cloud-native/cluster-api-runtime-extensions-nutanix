// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name         string
		spec         NutanixPrismCentralEndpointSpec
		expectedHost string
		expectedPort uint16
		expectedErr  error
	}{
		{
			name: "Valid URL with port",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			expectedHost: "192.168.1.1",
			expectedPort: 9440,
			expectedErr:  nil,
		},
		{
			name: "Valid URL without port",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1",
			},
			expectedHost: "192.168.1.1",
			expectedPort: 9440,
			expectedErr:  nil,
		},
		{
			name: "Invalid URL",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "invalid-url",
			},
			expectedHost: "",
			expectedPort: 0,
			expectedErr: fmt.Errorf(
				"error parsing Prism Central URL: parse %q: invalid URI for request",
				"invalid-url",
			),
		},
		{
			name: "Invalid port",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:invalid-port",
			},
			expectedHost: "",
			expectedPort: 0,
			expectedErr: fmt.Errorf(
				"error parsing Prism Central URL: parse %q: invalid port %q after host",
				"https://192.168.1.1:invalid-port",
				":invalid-port",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := tt.spec.ParseURL()
			if tt.expectedErr != nil {
				require.EqualError(t, err, tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestParseIP(t *testing.T) {
	tests := []struct {
		name        string
		spec        NutanixPrismCentralEndpointSpec
		expectedIP  netip.Addr
		expectedErr error
	}{
		{
			name: "Valid IP",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "https://192.168.1.1:9440",
			},
			expectedIP:  netip.MustParseAddr("192.168.1.1"),
			expectedErr: nil,
		},
		{
			name: "Invalid URL",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "invalid-url",
			},
			expectedIP: netip.Addr{},
			expectedErr: fmt.Errorf(
				"error parsing Prism Central URL: parse %q: invalid URI for request",
				"invalid-url",
			),
		},
		{
			name: "Invalid IP",
			spec: NutanixPrismCentralEndpointSpec{
				URL: "https://invalid-ip:9440",
			},
			expectedIP: netip.Addr{},
			expectedErr: fmt.Errorf(
				"error parsing Prism Central IP: ParseAddr(%q): unable to parse IP",
				"invalid-ip",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, err := tt.spec.ParseIP()
			if tt.expectedErr != nil {
				require.EqualError(t, err, tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}
