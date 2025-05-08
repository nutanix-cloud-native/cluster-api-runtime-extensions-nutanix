// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_ServiceIPForCluster(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cluster *clusterv1.Cluster
		want    string
	}{
		{
			name: "Cluster with nil service CIDR",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{},
				},
			},
			want: "10.96.0.20",
		},
		{
			name: "Cluster with empty service CIDR slice",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{},
						},
					},
				},
			},
			want: "10.96.0.20",
		},
		{
			name: "Cluster with a single service CIDR",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{
								"192.168.0.0/16",
							},
						},
					},
				},
			},
			want: "192.168.0.20",
		},
		{
			name: "Cluster with a multiple service CIDRs",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{
								"192.168.0.0/16",
								"10.96.0.0/12",
							},
						},
					},
				},
			},
			want: "192.168.0.20",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ServiceIPForCluster(tt.cluster)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
