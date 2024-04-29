// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagementCluster returns a Cluster object if c is pointing to a management cluster, otherwise returns nil.
func ManagementCluster(ctx context.Context, c client.Client) (*clusterv1.Cluster, error) {
	allNodes := &corev1.NodeList{}
	err := c.List(ctx, allNodes)
	if err != nil {
		return nil, fmt.Errorf("error listing Nodes: %w", err)
	}
	if len(allNodes.Items) == 0 {
		return nil, nil
	}
	annotations := allNodes.Items[0].Annotations
	clusterName := annotations[clusterv1.ClusterNameAnnotation]
	clusterNamespace := annotations[clusterv1.ClusterNamespaceAnnotation]
	if clusterName == "" && clusterNamespace == "" {
		return nil, nil
	}

	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Name:      clusterName,
		Namespace: clusterNamespace,
	}
	err = c.Get(ctx, key, cluster)
	if err != nil {
		if k8serrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting Cluster object based on Node annotations: %w", err)
	}

	return cluster, nil
}

func GetProvider(cluster *clusterv1.Cluster) string {
	if cluster == nil {
		return ""
	}
	return cluster.GetLabels()[clusterv1.ProviderNameLabel]
}
