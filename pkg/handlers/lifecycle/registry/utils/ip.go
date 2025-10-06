// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"

	netutils "k8s.io/utils/net"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const ipIndex = 20

func ServiceIPForCluster(cluster *clusterv1.Cluster) (string, error) {
	var serviceCIDRBlocks []string
	if cluster.Spec.ClusterNetwork != nil && cluster.Spec.ClusterNetwork.Services != nil {
		serviceCIDRBlocks = cluster.Spec.ClusterNetwork.Services.CIDRBlocks
	}
	serviceIP, err := getServiceIP(serviceCIDRBlocks)
	if err != nil {
		return "", fmt.Errorf("error getting a service IP for a cluster: %w", err)
	}

	return serviceIP, nil
}

func getServiceIP(serviceSubnetStrings []string) (string, error) {
	serviceSubnets, err := netutils.ParseCIDRs(serviceSubnetStrings)
	if err != nil {
		return "", fmt.Errorf("unable to parse service Subnets: %w", err)
	}
	if len(serviceSubnets) == 0 {
		return "", errors.New("unexpected empty service Subnets")
	}

	// Selects the 20th IP in service subnet CIDR range as the Service IP
	serviceIP, err := netutils.GetIndexedIP(serviceSubnets[0], ipIndex)
	if err != nil {
		return "", fmt.Errorf(
			"unable to get internal Kubernetes Service IP from the given service Subnets",
		)
	}

	return serviceIP.String(), nil
}
