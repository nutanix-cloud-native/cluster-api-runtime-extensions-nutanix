package utils

import (
	"errors"
	"fmt"

	netutils "k8s.io/utils/net"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func ServiceIPForCluster(cluster *clusterv1.Cluster) (string, error) {
	serviceIP, err := getServiceIP(cluster.Spec.ClusterNetwork.Services.CIDRBlocks)
	if err != nil {
		return "", fmt.Errorf("error getting a service IP for a cluster: %w", err)
	}

	return serviceIP, nil
}

func getServiceIP(serviceSubnetStrings []string) (string, error) {
	if len(serviceSubnetStrings) == 0 {
		serviceSubnetStrings = []string{v1alpha1.DefaultServicesSubnet}
	}

	serviceSubnets, err := netutils.ParseCIDRs(serviceSubnetStrings)
	if err != nil {
		return "", fmt.Errorf("unable to parse service Subnets: %w", err)
	}
	if len(serviceSubnets) == 0 {
		return "", errors.New("unexpected empty service Subnets")
	}

	// Selects the 20th IP in service subnet CIDR range as the Service IP
	serviceIP, err := netutils.GetIndexedIP(serviceSubnets[0], 20)
	if err != nil {
		return "", fmt.Errorf(
			"unable to get internal Kubernetes Service IP from the given service Subnets",
		)
	}

	return serviceIP.String(), nil
}
