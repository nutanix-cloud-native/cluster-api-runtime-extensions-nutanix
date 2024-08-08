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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/serviceloadbalancer/metallb"
)

type WaitForServiceLoadBalancerToBeReadyInWorkloadClusterInput struct {
	ServiceLoadBalancer  *v1alpha1.ServiceLoadBalancer
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	DeploymentIntervals  []interface{}
	DaemonSetIntervals   []interface{}
	HelmReleaseIntervals []interface{}
	ResourceIntervals    []interface{}
}

func WaitForServiceLoadBalancerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForServiceLoadBalancerToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.ServiceLoadBalancer == nil {
		return
	}

	switch providerName := input.ServiceLoadBalancer.Provider; providerName {
	case v1alpha1.ServiceLoadBalancerProviderMetalLB:
		waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadCluster(
			ctx,
			waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadClusterInput{
				workloadCluster:      input.WorkloadCluster,
				clusterProxy:         input.ClusterProxy,
				deploymentIntervals:  input.DeploymentIntervals,
				daemonSetIntervals:   input.DaemonSetIntervals,
				helmReleaseIntervals: input.HelmReleaseIntervals,
				resourceIntervals:    input.ResourceIntervals,
			},
		)
	default:
		Fail(
			fmt.Sprintf(
				"Do not know how to wait for ServiceLoadBalancer provider %s to be ready",
				providerName,
			),
		)
	}
}

type waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadClusterInput struct {
	workloadCluster      *clusterv1.Cluster
	clusterProxy         framework.ClusterProxy
	helmReleaseIntervals []interface{}
	deploymentIntervals  []interface{}
	daemonSetIntervals   []interface{}
	resourceIntervals    []interface{}
}

func waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadCluster(
	ctx context.Context,
	input waitForMetalLBServiceLoadBalancerToBeReadyInWorkloadClusterInput, //nolint:gocritic // OK in tests.
) {
	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:          input.clusterProxy.GetClient(),
			Cluster:            input.workloadCluster,
			HelmChartProxyName: "metallb-" + input.workloadCluster.Name,
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
				Name:      "metallb-controller",
				Namespace: "metallb-system",
			},
		},
	}, input.deploymentIntervals...)

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "metallb-speaker",
				Namespace: "metallb-system",
			},
		},
	}, input.daemonSetIntervals...)

	// Generate the MetalLB configuration objects, so we can wait for them to be
	// created on the workload cluster.
	cos, err := metallb.ConfigurationObjects(&metallb.ConfigurationInput{
		Name:      "metallb",
		Namespace: "metallb-system",
		// We need to populate AddressRanges to generate the configuration,
		// but the values are not important, because this test does not compare
		// them against the actual values.
		AddressRanges: []v1alpha1.AddressRange{
			{
				Start: "1.2.3.4",
				End:   "1.2.3.5",
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	resources := make([]client.Object, len(cos))
	for i := range cos {
		resources[i] = cos[i].DeepCopy()
	}

	WaitForResources(ctx, WaitForResourcesInput{
		Getter:    workloadClusterClient,
		Resources: resources,
	}, input.resourceIntervals...)
}
