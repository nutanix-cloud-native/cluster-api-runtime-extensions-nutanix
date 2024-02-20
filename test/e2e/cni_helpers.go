//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

func WaitForAddonsToBeReadyInWorkloadCluster(
	ctx context.Context,
	addonsConfig v1alpha1.Addons,
	workloadCluster *capiv1.Cluster,
	clusterProxy framework.ClusterProxy,
	deploymentIntervals []interface{},
	daemonSetIntervals []interface{},
) {
	WaitForCNIToBeReadyInWorkloadCluster(
		ctx,
		addonsConfig.CNI,
		workloadCluster,
		clusterProxy,
		deploymentIntervals,
		daemonSetIntervals,
	)
}

func WaitForCNIToBeReadyInWorkloadCluster(
	ctx context.Context,
	cni *v1alpha1.CNI,
	workloadCluster *capiv1.Cluster,
	clusterProxy framework.ClusterProxy,
	deploymentIntervals []interface{},
	daemonSetIntervals []interface{},
) {
	if cni == nil {
		return
	}

	switch cni.Provider {
	case v1alpha1.CNIProviderCalico:
		waitForCalicoToBeReadyInWorkloadCluster(
			ctx,
			cni.Strategy,
			workloadCluster,
			clusterProxy,
			deploymentIntervals,
			daemonSetIntervals,
		)
	case v1alpha1.CNIProviderCilium:
		waitForCiliumToBeReadyInWorkloadCluster(
			ctx,
			cni.Strategy,
			workloadCluster,
			clusterProxy,
			deploymentIntervals,
			daemonSetIntervals,
		)
	default:
		Fail(fmt.Sprintf("Do not know how to wait for CNI provider %s to be ready", cni.Provider))
	}
}

func waitForCalicoToBeReadyInWorkloadCluster(
	ctx context.Context,
	strategy v1alpha1.AddonStrategy,
	workloadCluster *capiv1.Cluster,
	clusterProxy framework.ClusterProxy,
	deploymentIntervals []interface{},
	daemonSetIntervals []interface{},
) {
	switch strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		waitForClusterResourceSetToApplyResourcesInCluster(
			ctx,
			workloadCluster.Namespace,
			"calico-cni-installation-"+workloadCluster.Name,
			clusterProxy,
			workloadCluster,
			deploymentIntervals...,
		)

		workloadClusterClient := clusterProxy.GetWorkloadCluster(
			ctx, workloadCluster.Namespace, workloadCluster.Name,
		).GetClient()

		WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: workloadClusterClient,
			Deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tigera-operator",
					Namespace: "tigera-operator",
				},
			},
		}, deploymentIntervals...)
		WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: workloadClusterClient,
			Deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "calico-typha",
					Namespace: "calico-system",
				},
			},
		}, deploymentIntervals...)
		WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: workloadClusterClient,
			Deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "calico-kube-controllers",
					Namespace: "calico-system",
				},
			},
		}, deploymentIntervals...)
		WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
			Getter: workloadClusterClient,
			DaemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "calico-node",
					Namespace: "calico-system",
				},
			},
		}, daemonSetIntervals...)
		WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
			Getter: workloadClusterClient,
			DaemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "csi-node-driver",
					Namespace: "calico-system",
				},
			},
		}, daemonSetIntervals...)
		WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: workloadClusterClient,
			Deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "calico-apiserver",
					Namespace: "calico-apiserver",
				},
			},
		}, deploymentIntervals...)
	case v1alpha1.AddonStrategyHelmAddon:
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for Calico using strategy %s to be ready",
				strategy,
			),
		)
	}
}

func waitForCiliumToBeReadyInWorkloadCluster(
	ctx context.Context,
	strategy v1alpha1.AddonStrategy,
	workloadCluster *capiv1.Cluster,
	clusterProxy framework.ClusterProxy,
	deploymentIntervals []interface{},
	daemonSetIntervals []interface{},
) {
	switch strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		waitForClusterResourceSetToApplyResourcesInCluster(
			ctx,
			workloadCluster.Namespace,
			"cilium-cni-installation-"+workloadCluster.Name,
			clusterProxy,
			workloadCluster,
			deploymentIntervals...,
		)

		workloadClusterClient := clusterProxy.GetWorkloadCluster(
			ctx, workloadCluster.Namespace, workloadCluster.Name,
		).GetClient()

		WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: workloadClusterClient,
			Deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-operator",
					Namespace: "kube-system",
				},
			},
		}, deploymentIntervals...)

		WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
			Getter: workloadClusterClient,
			DaemonSet: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium",
					Namespace: "kube-system",
				},
			},
		}, daemonSetIntervals...)
	case v1alpha1.AddonStrategyHelmAddon:
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for Cilium using strategy %s to be ready",
				strategy,
			),
		)
	}
}
