// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestReadinessSocketPath(t *testing.T) {
	tests := []struct {
		name          string
		cniProvider   string
		expectedPath  string
		expectedError bool
	}{
		{
			name:          "Cilium provider returns correct socket path",
			cniProvider:   v1alpha1.CNIProviderCilium,
			expectedPath:  "/run/cilium/cilium.sock",
			expectedError: false,
		},
		{
			name:          "Calico provider returns correct socket path",
			cniProvider:   v1alpha1.CNIProviderCalico,
			expectedPath:  "/var/run/calico/cni-server.sock",
			expectedError: false,
		},
		{
			name:          "Unsupported provider returns error",
			cniProvider:   "unsupported",
			expectedPath:  "",
			expectedError: true,
		},
		{
			name:          "Empty provider returns error",
			cniProvider:   "",
			expectedPath:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ReadinessSocketPath(tt.cniProvider)

			if tt.expectedError {
				require.Error(t, err)
				assert.Empty(t, path)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
}
