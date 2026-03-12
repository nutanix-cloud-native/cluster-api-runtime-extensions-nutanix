// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// RegistryAddon retrieves the RegistryAddon from the cluster's topology variables.
// Returns nil if the addon is not defined.
func RegistryAddon(cluster *clusterv1.Cluster) (*carenv1.RegistryAddon, error) {
	spec, err := UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable: %w", err)
	}
	if spec == nil {
		return nil, nil
	}
	if spec.Addons == nil {
		return nil, nil
	}

	return spec.Addons.Registry, nil
}

// KubeProxyMode retrieves the kube-proxy mode from the cluster's topology variables.
// Returns nil if the kube-proxy mode is not defined.
func KubeProxyMode(cluster *clusterv1.Cluster) (*carenv1.KubeProxyMode, error) {
	spec, err := UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster variable: %w", err)
	}
	if spec == nil {
		return nil, nil
	}
	if spec.KubeProxy == nil {
		return nil, nil
	}

	return &spec.KubeProxy.Mode, nil
}

// KubeProxyIsDisabled returns true if kube-proxy mode from the cluster's topology variables is disabled.
func KubeProxyIsDisabled(cluster *clusterv1.Cluster) (bool, error) {
	mode, err := KubeProxyMode(cluster)
	if err != nil {
		return false, err
	}
	return mode != nil && *mode == carenv1.KubeProxyModeDisabled, nil
}
