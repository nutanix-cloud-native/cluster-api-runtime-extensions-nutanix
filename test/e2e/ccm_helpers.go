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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

type WaitForCCMToBeReadyInWorkloadClusterInput struct {
	CCM                         *v1alpha1.CCM
	InfrastructureProvider      string
	WorkloadCluster             *clusterv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForCCMToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForCCMToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.CCM == nil {
		return
	}

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	switch input.InfrastructureProvider {
	case "aws":
		WaitForAWSCCMToBeReadyInWorkloadCluster(
			ctx,
			workloadClusterClient,
			input,
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for CCM using infrastructure provider %s to be ready",
				input.InfrastructureProvider,
			),
		)
	}
}

func WaitForAWSCCMToBeReadyInWorkloadCluster(
	ctx context.Context,
	workloadClusterClient client.Client,
	input WaitForCCMToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	switch input.CCM.Strategy {
	case v1alpha1.AddonStrategyClusterResourceSet:
		crs := &addonsv1.ClusterResourceSet{}
		Expect(input.ClusterProxy.GetClient().Get(
			ctx,
			types.NamespacedName{
				Name:      "aws-ccm-" + input.WorkloadCluster.Name,
				Namespace: input.WorkloadCluster.Namespace,
			},
			crs,
		)).To(Succeed())

		framework.WaitForClusterResourceSetToApplyResources(
			ctx,
			framework.WaitForClusterResourceSetToApplyResourcesInput{
				ClusterResourceSet: crs,
				ClusterProxy:       input.ClusterProxy,
				Cluster:            input.WorkloadCluster,
			},
			input.ClusterResourceSetIntervals...,
		)
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:          input.ClusterProxy.GetClient(),
				Cluster:            input.WorkloadCluster,
				HelmChartProxyName: "aws-cloud-controller-manager-" + input.WorkloadCluster.Name,
			},
			input.HelmReleaseIntervals...,
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for AWS CCM using strategy %s to be ready",
				input.CCM.Strategy,
			),
		)
	}

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "aws-cloud-controller-manager",
				Namespace: metav1.NamespaceSystem,
			},
		},
	}, input.DaemonSetIntervals...)
}
