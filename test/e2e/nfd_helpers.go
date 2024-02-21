//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

type WaitForNFDToBeReadyInWorkloadClusterInput struct {
	NFD                         *v1alpha1.NFD
	WorkloadCluster             *capiv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForNFDToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForNFDToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.NFD == nil {
		return
	}

	waitForClusterResourceSetToApplyResourcesInCluster(
		ctx,
		waitForClusterResourceSetToApplyResourcesInClusterInput{
			name:         "node-feature-discovery-" + input.WorkloadCluster.Name,
			clusterProxy: input.ClusterProxy,
			cluster:      input.WorkloadCluster,
			intervals:    input.ClusterResourceSetIntervals,
		},
	)

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-feature-discovery-gc",
				Namespace: "node-feature-discovery",
			},
		},
	}, input.DeploymentIntervals...)

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-feature-discovery-master",
				Namespace: "node-feature-discovery",
			},
		},
	}, input.DeploymentIntervals...)

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "node-feature-discovery-worker",
				Namespace: "node-feature-discovery",
			},
		},
	}, input.DaemonSetIntervals...)
}
