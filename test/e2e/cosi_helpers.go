//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type WaitForCOSIControllerToBeReadyInWorkloadClusterInput struct {
	COSI                 *apivariables.COSI
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	DeploymentIntervals  []interface{}
	HelmReleaseIntervals []interface{}
}

func WaitForCOSIControllerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForCOSIControllerToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.COSI == nil {
		return
	}

	switch ptr.Deref(input.COSI.Strategy, "") {
	case v1alpha1.AddonStrategyHelmAddon:
		WaitForHelmReleaseProxyReadyForCluster(
			ctx,
			WaitForHelmReleaseProxyReadyForClusterInput{
				GetLister:       input.ClusterProxy.GetClient(),
				Cluster:         input.WorkloadCluster,
				HelmReleaseName: "cosi-controller",
			},
			input.HelmReleaseIntervals...,
		)
	case "":
		Fail("COSI strategy is not set")
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for COSI using strategy %s to be ready",
				*input.COSI.Strategy,
			),
		)
	}

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: workloadClusterClient,
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "container-object-storage-controller",
				Namespace: "container-object-storage-system",
			},
		},
	}, input.DeploymentIntervals...)
}
