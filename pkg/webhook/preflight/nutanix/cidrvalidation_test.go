// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/preflight"
)

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

func TestCIDRValidationCheckRun_SizeValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		podCIDRs           []string
		serviceCIDRs       []string
		expectAllowed      bool
		expectedCauseParts []string
		expectedWarnings   int
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
			expectedCauseParts: []string{
				"Pod CIDR \"10.244.0.0/24\" has prefix /24, which is too small for multi-node clusters",
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
			expectedCauseParts: []string{
				"Service CIDR \"10.96.0.0/24\" is too small",
			},
			expectedWarnings: 0,
		},
		{
			name:          "service CIDR /25 blocks deployment",
			podCIDRs:      []string{"10.244.0.0/16"},
			serviceCIDRs:  []string{"10.96.0.0/25"},
			expectAllowed: false,
			expectedCauseParts: []string{
				"Service CIDR \"10.96.0.0/25\" is too small",
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
			name:               "invalid pod cidr",
			podCIDRs:           []string{"not-a-cidr"},
			serviceCIDRs:       []string{"10.96.0.0/12"},
			expectAllowed:      false,
			expectedCauseParts: []string{"Invalid Pod CIDR configuration"},
		},
		{
			name:               "invalid service cidr",
			podCIDRs:           []string{"10.244.0.0/16"},
			serviceCIDRs:       []string{"not-a-cidr"},
			expectAllowed:      false,
			expectedCauseParts: []string{"Invalid Service CIDR configuration"},
		},
		{
			name:          "overlapping Pod and Service CIDRs",
			podCIDRs:      []string{"10.0.0.0/16"},
			serviceCIDRs:  []string{"10.0.0.0/24"},
			expectAllowed: false,
			expectedCauseParts: []string{
				"Pod CIDR \"10.0.0.0/16\" overlaps with Service CIDR \"10.0.0.0/24\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cd := &checkDependencies{
				cluster: testCluster(tt.podCIDRs, tt.serviceCIDRs),
			}

			check := &cidrValidationCheck{
				cd: cd,
			}

			result := check.Run(context.Background())

			assert.Equal(t, tt.expectAllowed, result.Allowed, "Allowed mismatch")
			assert.Len(t, result.Warnings, tt.expectedWarnings, "Warnings count mismatch")

			for _, part := range tt.expectedCauseParts {
				assert.Contains(
					t,
					flattenCauseMessages(result.Causes),
					part,
					"expected cause to contain %q",
					part,
				)
			}
		})
	}
}

func testCluster(podCIDRs, serviceCIDRs []string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: &clusterv1.ClusterNetwork{
				Pods: &clusterv1.NetworkRanges{
					CIDRBlocks: podCIDRs,
				},
				Services: &clusterv1.NetworkRanges{
					CIDRBlocks: serviceCIDRs,
				},
			},
		},
	}
}

func flattenCauseMessages(causes []preflight.Cause) string {
	messages := make([]string, 0, len(causes))
	for _, cause := range causes {
		messages = append(messages, cause.Message)
	}
	return fmt.Sprintf("%v", messages)
}
