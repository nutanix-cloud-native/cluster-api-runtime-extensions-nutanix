//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"

	corednsversions "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/versions"
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

// WaitForDNSUpgradeInput is the input for WaitForDNSUpgrade.
type WaitForDNSUpgradeInput struct {
	WorkloadCluster     *clusterv1.Cluster
	ClusterProxy        framework.ClusterProxy
	DeploymentIntervals []interface{}
}

// WaitForCoreDNSImageVersion waits for the CoreDNS image to be updated to the default version,
// based on the Kubernetes version.
func WaitForCoreDNSImageVersion(
	ctx context.Context,
	input WaitForDNSUpgradeInput,
) {
	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	defaultCoreDNSVersion, found := corednsversions.GetCoreDNSVersion(
		input.WorkloadCluster.Spec.Topology.Version,
	)
	Expect(found).To(
		BeTrue(),
		"failed to get default CoreDNS version for Cluster version %s",
		input.WorkloadCluster.Spec.Topology.Version,
	)

	framework.WaitForDNSUpgrade(ctx, framework.WaitForDNSUpgradeInput{
		Getter:     workloadClusterClient,
		DNSVersion: defaultCoreDNSVersion,
	})
}
