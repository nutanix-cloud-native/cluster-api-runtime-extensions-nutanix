// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

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
