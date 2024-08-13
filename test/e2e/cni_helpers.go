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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type WaitForCNIToBeReadyInWorkloadClusterInput struct {
	CNI                         *v1alpha1.CNI
	WorkloadCluster             *clusterv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForCNIToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForCNIToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.CNI == nil {
		return
	}

	switch input.CNI.Provider {
	case v1alpha1.CNIProviderCalico:
		waitForCalicoToBeReadyInWorkloadCluster(
			ctx,
			waitForCalicoToBeReadyInWorkloadClusterInput{
				strategy:                    ptr.Deref(input.CNI.Strategy, ""),
				workloadCluster:             input.WorkloadCluster,
				clusterProxy:                input.ClusterProxy,
				deploymentIntervals:         input.DeploymentIntervals,
				daemonSetIntervals:          input.DaemonSetIntervals,
				helmReleaseIntervals:        input.HelmReleaseIntervals,
				clusterResourceSetIntervals: input.ClusterResourceSetIntervals,
			},
		)
	case v1alpha1.CNIProviderCilium:
		waitForCiliumToBeReadyInWorkloadCluster(
			ctx,
			waitForCiliumToBeReadyInWorkloadClusterInput{
				strategy:                    ptr.Deref(input.CNI.Strategy, ""),
				workloadCluster:             input.WorkloadCluster,
				clusterProxy:                input.ClusterProxy,
				deploymentIntervals:         input.DeploymentIntervals,
				daemonSetIntervals:          input.DaemonSetIntervals,
				helmReleaseIntervals:        input.HelmReleaseIntervals,
				clusterResourceSetIntervals: input.ClusterResourceSetIntervals,
			},
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for CNI provider %s to be ready",
				input.CNI.Provider,
			),
		)
	}
}

type waitForCalicoToBeReadyInWorkloadClusterInput struct {
	strategy                    v1alpha1.AddonStrategy
	workloadCluster             *clusterv1.Cluster
	clusterProxy                framework.ClusterProxy
	deploymentIntervals         []interface{}
	daemonSetIntervals          []interface{}
	helmReleaseIntervals        []interface{}
	clusterResourceSetIntervals []interface{}
}

func waitForCalicoToBeReadyInWorkloadCluster(
	ctx context.Context,
	input waitForCalicoToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	switch input.strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		crs := &addonsv1.ClusterResourceSet{}
		Expect(input.clusterProxy.GetClient().Get(
			ctx,
			types.NamespacedName{
				Name:      "calico-cni-installation-" + input.workloadCluster.Name,
				Namespace: input.workloadCluster.Namespace,
			},
			crs,
		)).To(Succeed())

		framework.WaitForClusterResourceSetToApplyResources(
			ctx,
			framework.WaitForClusterResourceSetToApplyResourcesInput{
				ClusterResourceSet: crs,
				ClusterProxy:       input.clusterProxy,
				Cluster:            input.workloadCluster,
			},
			input.clusterResourceSetIntervals...,
		)
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:       input.clusterProxy.GetClient(),
				Cluster:         input.workloadCluster,
				HelmReleaseName: "tigera-operator",
			},
			input.helmReleaseIntervals...,
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for Calico using strategy %s to be ready",
				input.strategy,
			),
		)
	}

	workloadClusterClient := input.clusterProxy.GetWorkloadCluster(
		ctx, input.workloadCluster.Namespace, input.workloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tigera-operator",
				Namespace: "tigera-operator",
			},
		},
	}, input.deploymentIntervals...)
	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "calico-typha",
				Namespace: "calico-system",
			},
		},
	}, input.deploymentIntervals...)
	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "calico-kube-controllers",
				Namespace: "calico-system",
			},
		},
	}, input.deploymentIntervals...)
	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "calico-node",
				Namespace: "calico-system",
			},
		},
	}, input.daemonSetIntervals...)
	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "csi-node-driver",
				Namespace: "calico-system",
			},
		},
	}, input.daemonSetIntervals...)
	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "calico-apiserver",
				Namespace: "calico-apiserver",
			},
		},
	}, input.deploymentIntervals...)
}

type waitForCiliumToBeReadyInWorkloadClusterInput struct {
	strategy                    v1alpha1.AddonStrategy
	workloadCluster             *clusterv1.Cluster
	clusterProxy                framework.ClusterProxy
	deploymentIntervals         []interface{}
	daemonSetIntervals          []interface{}
	helmReleaseIntervals        []interface{}
	clusterResourceSetIntervals []interface{}
}

func waitForCiliumToBeReadyInWorkloadCluster(
	ctx context.Context,
	input waitForCiliumToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	switch input.strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		crs := &addonsv1.ClusterResourceSet{}
		Expect(input.clusterProxy.GetClient().Get(
			ctx,
			types.NamespacedName{
				Name:      "cilium-cni-installation-" + input.workloadCluster.Name,
				Namespace: input.workloadCluster.Namespace,
			},
			crs,
		)).To(Succeed())

		framework.WaitForClusterResourceSetToApplyResources(
			ctx,
			framework.WaitForClusterResourceSetToApplyResourcesInput{
				ClusterResourceSet: crs,
				ClusterProxy:       input.clusterProxy,
				Cluster:            input.workloadCluster,
			},
			input.clusterResourceSetIntervals...,
		)
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:       input.clusterProxy.GetClient(),
				Cluster:         input.workloadCluster,
				HelmReleaseName: "cilium",
			},
			input.helmReleaseIntervals...,
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for Cilium using strategy %s to be ready",
				input.strategy,
			),
		)
	}

	workloadClusterClient := input.clusterProxy.GetWorkloadCluster(
		ctx, input.workloadCluster.Namespace, input.workloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cilium-operator",
				Namespace: "kube-system",
			},
		},
	}, input.deploymentIntervals...)

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cilium",
				Namespace: "kube-system",
			},
		},
	}, input.daemonSetIntervals...)
}
