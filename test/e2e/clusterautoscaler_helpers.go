//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

const clusterAutoscalerPrefix = "cluster-autoscaler-"

type WaitForClusterAutoscalerToBeReadyInWorkloadClusterInput struct {
	ClusterAutoscaler           *v1alpha1.ClusterAutoscaler
	WorkloadCluster             *clusterv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForClusterAutoscalerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForClusterAutoscalerToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.ClusterAutoscaler == nil {
		return
	}

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()
	// Only check for ClusterAutoscaler if the cluster is self-managed.
	// managementCluster will be nil if workloadClusterClient is not a self-managed cluster.
	managementCluster, err := utils.ManagementCluster(ctx, workloadClusterClient)
	Expect(err).NotTo(HaveOccurred())
	if managementCluster == nil {
		return
	}

	switch input.ClusterAutoscaler.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		waitForClusterResourceSetToApplyResourcesInCluster(
			ctx,
			waitForClusterResourceSetToApplyResourcesInClusterInput{
				name:         clusterAutoscalerPrefix + input.WorkloadCluster.Name,
				clusterProxy: input.ClusterProxy,
				cluster:      input.WorkloadCluster,
				intervals:    input.ClusterResourceSetIntervals,
			},
		)
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:          input.ClusterProxy.GetClient(),
				Cluster:            input.WorkloadCluster,
				HelmChartProxyName: clusterAutoscalerPrefix + input.WorkloadCluster.Name,
			},
			input.HelmReleaseIntervals...,
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for cluster autoscaler using strategy %s to be ready",
				input.ClusterAutoscaler.Strategy,
			),
		)
	}

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterAutoscalerPrefix + input.WorkloadCluster.Name,
				Namespace: input.WorkloadCluster.Namespace,
			},
		},
	}, input.DeploymentIntervals...)
}
