// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func Test_PodCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *clusterv1.Cluster
		wantCIDR string
		wantErr  error
	}{
		{
			name: "no Pods CIDR set",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{},
			},
		},
		{
			name: "no Pods CIDR set, but Services CIDR is set",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Services: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
					},
				},
			},
		},
		{
			name: "Pods CIDR set",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
					},
				},
			},
			wantCIDR: "192.168.0.1/16",
		},
		{
			name: "error: multiple Pods CIDRs set",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16", "10.0.0.1/16"},
						},
					},
				},
			},
			wantErr: ErrMultiplePodsCIDRBlocks,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cidr, err := PodCIDR(tt.cluster)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantCIDR, cidr)
		})
	}
}
