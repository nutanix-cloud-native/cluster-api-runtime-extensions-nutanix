//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type WaitForRegistryAddonToBeReadyInWorkloadClusterInput struct {
	Registry             *v1alpha1.RegistryAddon
	WorkloadCluster      *clusterv1.Cluster
	ClusterProxy         framework.ClusterProxy
	StatefulSetIntervals []interface{}
	HelmReleaseIntervals []interface{}
}

func WaitForRegistryAddonToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForRegistryAddonToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	if input.Registry == nil {
		return
	}

	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.ClusterProxy.GetClient(),
			Cluster:         input.WorkloadCluster,
			HelmReleaseName: "cncf-distribution-registry",
		},
		input.HelmReleaseIntervals...,
	)

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx, input.WorkloadCluster.Namespace, input.WorkloadCluster.Name,
	).GetClient()

	WaitForStatefulSetsAvailable(ctx, WaitForStatefulSetAvailableInput{
		Getter: workloadClusterClient,
		StatefulSet: &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cncf-distribution-registry-docker-registry",
				Namespace: "registry-system",
			},
		},
	}, input.StatefulSetIntervals...)
}

type EnsureClusterCAForRegistryAddonInput struct {
	Registry        *v1alpha1.RegistryAddon
	WorkloadCluster *clusterv1.Cluster
	ClusterProxy    framework.ClusterProxy
}

// EnsureClusterCAForRegistryAddon verifies that the cluster CA data exists and matches the root CA.
func EnsureClusterCAForRegistryAddon(
	ctx context.Context,
	input EnsureClusterCAForRegistryAddonInput,
) {
	if input.Registry == nil {
		return
	}

	cl := input.ClusterProxy.GetClient()

	rootCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      handlersutils.RegistryAddonRootCASecretName,
			Namespace: input.WorkloadCluster.Namespace,
		},
	}
	err := cl.Get(ctx, ctrlclient.ObjectKeyFromObject(rootCASecret), rootCASecret)
	Expect(err).NotTo(HaveOccurred())
	Expect(rootCASecret.Data).ToNot(BeEmpty())

	clusterCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      handlersutils.SecretNameForRegistryAddonCA(input.WorkloadCluster),
			Namespace: input.WorkloadCluster.Namespace,
		},
	}
	err = cl.Get(ctx, ctrlclient.ObjectKeyFromObject(clusterCASecret), clusterCASecret)
	Expect(err).NotTo(HaveOccurred())
	Expect(clusterCASecret.Data).ToNot(BeEmpty())

	const caCrtKey = "ca.crt"
	Expect(rootCASecret.Data[caCrtKey]).To(Equal(rootCASecret.Data[caCrtKey]))
}
