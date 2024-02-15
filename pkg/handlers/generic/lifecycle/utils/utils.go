// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	crsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
)

func EnsureCRSForClusterFromConfigMaps(
	ctx context.Context,
	crsName string,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	configMaps ...*corev1.ConfigMap,
) error {
	resources := make([]crsv1.ResourceRef, 0, len(configMaps))
	for _, cm := range configMaps {
		resources = append(resources, crsv1.ResourceRef{
			Kind: string(crsv1.ConfigMapClusterResourceSetResourceKind),
			Name: cm.Name,
		})
	}

	crs := &crsv1.ClusterResourceSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: crsv1.GroupVersion.String(),
			Kind:       "ClusterResourceSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      crsName,
		},
		Spec: crsv1.ClusterResourceSetSpec{
			Resources: resources,
			Strategy:  string(crsv1.ClusterResourceSetStrategyReconcile),
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(cluster, crs, c.Scheme()); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	err := client.ServerSideApply(ctx, c, crs)
	if err != nil {
		return fmt.Errorf("failed to server side apply %w", err)
	}

	return nil
}
