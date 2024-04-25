// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/cluster-api/api/v1beta1"
)

func Test_PodCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cluster  *v1beta1.Cluster
		wantCIDR string
		wantErr  error
	}{
		{
			name: "no Pods CIDR set",
			cluster: &v1beta1.Cluster{
				Spec: v1beta1.ClusterSpec{},
			},
		},
		{
			name: "no Pods CIDR set, but Services CIDR is set",
			cluster: &v1beta1.Cluster{
				Spec: v1beta1.ClusterSpec{
					ClusterNetwork: &v1beta1.ClusterNetwork{
						Services: &v1beta1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
					},
				},
			},
		},
		{
			name: "Pods CIDR set",
			cluster: &v1beta1.Cluster{
				Spec: v1beta1.ClusterSpec{
					ClusterNetwork: &v1beta1.ClusterNetwork{
						Pods: &v1beta1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
					},
				},
			},
			wantCIDR: "192.168.0.1/16",
		},
		{
			name: "error: multiple Pods CIDRs set",
			cluster: &v1beta1.Cluster{
				Spec: v1beta1.ClusterSpec{
					ClusterNetwork: &v1beta1.ClusterNetwork{
						Pods: &v1beta1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16", "10.0.0.1/16"},
						},
					},
				},
			},
			wantErr: ErrMultiplePodsCIDRBlocks,
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cidr, err := PodCIDR(tt.cluster)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantCIDR, cidr)
		})
	}
}
