//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type WaitForMultusToBeReadyInWorkloadClusterInput struct {
	CNI                    *v1alpha1.CNI
	WorkloadCluster        *clusterv1.Cluster
	InfrastructureProvider string
	ClusterProxy           framework.ClusterProxy
	DaemonSetIntervals     []interface{}
	HelmReleaseIntervals   []interface{}
}

// WaitForMultusToBeReadyInWorkloadCluster verifies that Multus is deployed and ready
// in the workload cluster. Multus is auto-deployed for EKS and Nutanix clusters
// when a supported CNI (Cilium or Calico) is configured.
func WaitForMultusToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForMultusToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	// Multus is only auto-deployed for EKS (AWS) and Nutanix clusters
	if input.InfrastructureProvider != "aws" && input.InfrastructureProvider != "nutanix" {
		return
	}

	// Multus requires a supported CNI provider (Cilium or Calico)
	if input.CNI == nil {
		return
	}

	if input.CNI.Provider != v1alpha1.CNIProviderCilium && input.CNI.Provider != v1alpha1.CNIProviderCalico {
		return
	}

	By("Waiting for Multus HelmChartProxy to be ready")
	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.ClusterProxy.GetClient(),
			Cluster:         input.WorkloadCluster,
			HelmReleaseName: "multus",
		},
		input.HelmReleaseIntervals...,
	)

	By("Waiting for Multus DaemonSet to be available")
	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx,
		input.WorkloadCluster.Namespace,
		input.WorkloadCluster.Name,
	).GetClient()

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multus",
				Namespace: metav1.NamespaceSystem,
			},
		},
	}, input.DaemonSetIntervals...)

	By("Verifying Multus DaemonSet is running")
	multusDaemonSet := &appsv1.DaemonSet{}
	err := workloadClusterClient.Get(
		ctx,
		types.NamespacedName{
			Name:      "multus",
			Namespace: metav1.NamespaceSystem,
		},
		multusDaemonSet,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(multusDaemonSet.Status.NumberAvailable).To(BeNumerically(">", 0))
	Expect(multusDaemonSet.Status.NumberUnavailable).To(Equal(int32(0)))
}
