//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"

	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type WaitForKonnectorAgentToBeReadyInWorkloadClusterInput struct {
	KonnectorAgent       *apivariables.NutanixKonnectorAgent
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	DeploymentIntervals  []interface{}
	HelmReleaseIntervals []interface{}
}

func WaitForKonnectorAgentToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForKonnectorAgentToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.KonnectorAgent == nil {
		return
	}

	// Wait for HelmReleaseProxy to be ready
	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.ClusterProxy.GetClient(),
			Cluster:         input.WorkloadCluster,
			HelmReleaseName: "konnector-agent",
		},
		input.HelmReleaseIntervals...,
	)

	// Get workload cluster client to check resources in the workload cluster
	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	// Wait for konnector-agent deployment to be available
	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "konnector-agent",
				Namespace: "ntnx-system",
			},
		},
	}, input.DeploymentIntervals...)
}
