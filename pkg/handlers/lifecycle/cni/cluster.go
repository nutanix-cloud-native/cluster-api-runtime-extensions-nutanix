// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"errors"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var ErrMultiplePodsCIDRBlocks = errors.New("cluster has more than 1 Pods network CIDR blocks")

// PodCIDR will return the Pods network CIDR.
// If not set returns an empty string.
// If more than 1 CIDRBlocks is defined will return an error.
func PodCIDR(cluster *clusterv1.Cluster) (string, error) {
	var subnets []string
	if cluster.Spec.ClusterNetwork != nil &&
		cluster.Spec.ClusterNetwork.Pods != nil {
		subnets = cluster.Spec.ClusterNetwork.Pods.CIDRBlocks
	}
	switch {
	case len(subnets) == 1:
		return cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0], nil
	case len(subnets) > 1:
		return "", ErrMultiplePodsCIDRBlocks
	default:
		return "", nil
	}
}
