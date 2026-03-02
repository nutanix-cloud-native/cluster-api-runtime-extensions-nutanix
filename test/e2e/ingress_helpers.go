//go:build e2e

// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type WaitForIngressToBeReadyInWorkloadClusterInput struct {
	Ingress              *apivariables.Ingress
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	DeploymentIntervals  []interface{}
	HelmReleaseIntervals []interface{}
}

func WaitForIngressToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForIngressToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.Ingress == nil {
		return
	}

	switch input.Ingress.Provider {
	case v1alpha1.IngressProviderAWSLoadBalancerController:
		waitForAWSLoadBalancerControllerToBeReadyInWorkloadCluster(
			ctx,
			waitForAWSLoadBalancerControllerToBeReadyInWorkloadClusterInput{
				workloadCluster:      input.WorkloadCluster,
				clusterProxy:         input.ClusterProxy,
				deploymentIntervals:  input.DeploymentIntervals,
				helmReleaseIntervals: input.HelmReleaseIntervals,
			},
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for Ingress provider %s to be ready",
				input.Ingress.Provider,
			),
		)
	}
}

type waitForAWSLoadBalancerControllerToBeReadyInWorkloadClusterInput struct {
	workloadCluster      *clusterv1.Cluster
	clusterProxy         framework.ClusterProxy
	deploymentIntervals  []interface{}
	helmReleaseIntervals []interface{}
}

func waitForAWSLoadBalancerControllerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input waitForAWSLoadBalancerControllerToBeReadyInWorkloadClusterInput,
) {
	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.clusterProxy.GetClient(),
			Cluster:         input.workloadCluster,
			HelmReleaseName: "aws-lb-controller",
		},
		input.helmReleaseIntervals...,
	)

	workloadClusterClient := input.clusterProxy.GetWorkloadCluster(
		ctx, input.workloadCluster.Namespace, input.workloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aws-load-balancer-controller",
				Namespace: "kube-system",
			},
		},
	}, input.deploymentIntervals...)
}
