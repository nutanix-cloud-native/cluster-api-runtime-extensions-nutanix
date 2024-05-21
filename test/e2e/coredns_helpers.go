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
)

type WaitForCoreDNSToBeReadyInWorkloadClusterInput struct {
	WorkloadCluster     *clusterv1.Cluster
	ClusterProxy        framework.ClusterProxy
	DeploymentIntervals []interface{}
}

func WaitForCoreDNSToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForCoreDNSToBeReadyInWorkloadClusterInput,
) {
	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: metav1.NamespaceSystem,
			},
		},
	}, input.DeploymentIntervals...)
}
