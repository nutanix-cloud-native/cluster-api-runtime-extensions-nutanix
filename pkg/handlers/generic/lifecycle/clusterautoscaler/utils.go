// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"context"
	"fmt"

	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

// findTargetCluster returns the management Cluster if it exists, otherwise returns the given cluster.
func findTargetCluster(
	ctx context.Context,
	c client.Client,
	cluster *v1beta1.Cluster,
) (*v1beta1.Cluster, error) {
	existingManagementCluster, err := capiutils.ManagementCluster(ctx, c)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get management Cluster: %w",
			err,
		)
	}

	// In most cases, target the management cluster.
	// But if existingManagementCluster is nil, i.e. when c points to a bootstrap cluster,
	// target the cluster and assume that will become the management cluster.
	targetCluster := existingManagementCluster
	if targetCluster == nil {
		targetCluster = cluster
	}

	return targetCluster, nil
}
