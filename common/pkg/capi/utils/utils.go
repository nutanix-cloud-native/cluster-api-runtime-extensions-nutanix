// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagementCluster returns a Cluster object if c is pointing to a management cluster, otherwise returns nil.
func ManagementCluster(ctx context.Context, c client.Reader) (*clusterv1.Cluster, error) {
	clusterName, clusterNamespace, err := clusterAnnotationsFromNodes(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster annotations from Nodes: %w", err)
	}

	if clusterName == "" && clusterNamespace == "" {
		return nil, nil
	}

	cluster, err := managementClusterFromNodeAnnotations(ctx, c, clusterName, clusterNamespace)
	if err != nil {
		if k8serrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}

	return cluster, nil
}

// ManagementOrFutureManagementCluster returns a Cluster object to either the management cluster,
// when c is a client to the management cluster.
// Or a cluster that is assumed to become the management cluster in the future,
// when is c is a client to a bootstrap cluster and there is only a single Cluster object.
func ManagementOrFutureManagementCluster(ctx context.Context, c client.Reader) (*clusterv1.Cluster, error) {
	clusterName, clusterNamespace, err := clusterAnnotationsFromNodes(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster annotations from Nodes: %w", err)
	}

	var cluster *clusterv1.Cluster
	switch {
	case clusterName != "" && clusterNamespace != "":
		cluster, err = managementClusterFromNodeAnnotations(ctx, c, clusterName, clusterNamespace)
	case clusterName == "" && clusterNamespace == "":
		cluster, err = managementClusterFromBootstrapClient(ctx, c)
	case clusterName == "":
		err = fmt.Errorf("missing %q annotation", clusterv1.ClusterNameAnnotation)
	case clusterNamespace == "":
		err = fmt.Errorf("missing %q annotation", clusterv1.ClusterNamespaceAnnotation)
	}
	if err != nil {
		return nil, fmt.Errorf("error determining management cluster for the provided client: %w", err)
	}

	return cluster, nil
}

func clusterAnnotationsFromNodes(
	ctx context.Context,
	c client.Reader,
) (clusterName, clusterNamespace string, err error) {
	allNodes := &corev1.NodeList{}
	if err := c.List(ctx, allNodes); err != nil {
		return "", "", fmt.Errorf("error listing Nodes: %w", err)
	}

	// Get node annotations that should exist in the management cluster.
	annotations := make(map[string]string)
	if len(allNodes.Items) > 0 {
		annotations = allNodes.Items[0].Annotations
	}
	return annotations[clusterv1.ClusterNameAnnotation], annotations[clusterv1.ClusterNamespaceAnnotation], nil
}

func managementClusterFromNodeAnnotations(
	ctx context.Context,
	c client.Reader,
	clusterName, clusterNamespace string,
) (*clusterv1.Cluster, error) {
	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Name:      clusterName,
		Namespace: clusterNamespace,
	}
	err := c.Get(ctx, key, cluster)
	if err != nil {
		return nil, fmt.Errorf("error getting Cluster object based on Node annotations: %w", err)
	}

	return cluster, nil
}

// managementClusterFromBootstrapClient returns a Cluster object that is assumed to become the management cluster.
// Returns an error if there is not exactly one Cluster object in the cluster.
func managementClusterFromBootstrapClient(
	ctx context.Context,
	c client.Reader,
) (*clusterv1.Cluster, error) {
	clusters := &clusterv1.ClusterList{}
	err := c.List(ctx, clusters)
	if err != nil {
		return nil, fmt.Errorf("error listing Clusters: %w", err)
	}

	switch {
	case len(clusters.Items) == 0:
		return nil, fmt.Errorf("no Cluster objects found")
	case len(clusters.Items) > 1:
		return nil, fmt.Errorf("multiple Cluster objects found, expected exactly one")
	default:
		return &clusters.Items[0], nil
	}
}

func GetProvider(cluster *clusterv1.Cluster) string {
	if cluster == nil {
		return ""
	}
	return cluster.GetLabels()[clusterv1.ProviderNameLabel]
}
