//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type WaitForRegistryMirrorToBeReadyInWorkloadClusterInput struct {
	RegistryMirror       *v1alpha1.RegistryMirror
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	StatefulSetIntervals []interface{}
	HelmReleaseIntervals []interface{}
}

func WaitForRegistryMirrorToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForRegistryMirrorToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.RegistryMirror == nil {
		return
	}

	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.ClusterProxy.GetClient(),
			Cluster:         input.WorkloadCluster,
			HelmReleaseName: "registry-mirror",
		},
		input.HelmReleaseIntervals...,
	)

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	WaitForStatefulSetsAvailable(ctx, WaitForStatefulSetAvailableInput{
		Getter: workloadClusterClient,
		StatefulSet: &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "registry-mirror-docker-registry",
				Namespace: "registry-mirror-system",
			},
		},
	}, input.StatefulSetIntervals...)
}
