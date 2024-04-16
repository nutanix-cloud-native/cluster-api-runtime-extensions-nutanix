//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type waitForClusterResourceSetToApplyResourcesInClusterInput struct {
	name         string
	clusterProxy framework.ClusterProxy
	cluster      *clusterv1.Cluster
	intervals    []interface{}
}

func waitForClusterResourceSetToApplyResourcesInCluster(
	ctx context.Context,
	input waitForClusterResourceSetToApplyResourcesInClusterInput,
) {
	crs := &addonsv1.ClusterResourceSet{}
	Expect(input.clusterProxy.GetClient().Get(
		ctx, types.NamespacedName{Name: input.name, Namespace: input.cluster.Namespace}, crs,
	)).To(Succeed())

	waitForClusterResourceSetToApplyResources(
		ctx,
		framework.WaitForClusterResourceSetToApplyResourcesInput{
			ClusterProxy:       input.clusterProxy,
			Cluster:            input.cluster,
			ClusterResourceSet: crs,
		},
		input.intervals...,
	)
}

// Copied from upstream to fix bug checking bindings. Need to contribute fix back upstream and use it from there once
// available.
//
// waitForClusterResourceSetToApplyResources wait until all ClusterResourceSet resources are created in the matching
// cluster.
func waitForClusterResourceSetToApplyResources(
	ctx context.Context,
	input framework.WaitForClusterResourceSetToApplyResourcesInput,
	intervals ...interface{},
) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for WaitForClusterResourceSetToApplyResources")
	Expect(
		input.ClusterProxy,
	).ToNot(
		BeNil(),
		"Invalid argument. input.ClusterProxy can't be nil when calling WaitForClusterResourceSetToApplyResources",
	)
	Expect(
		input.Cluster,
	).ToNot(
		BeNil(),
		"Invalid argument. input.Cluster can't be nil when calling WaitForClusterResourceSetToApplyResources",
	)
	Expect(
		input.ClusterResourceSet,
	).NotTo(
		BeNil(),
		"Invalid argument. input.ClusterResourceSet can't be nil when calling WaitForClusterResourceSetToApplyResources",
	)

	fmt.Fprintln(GinkgoWriter, "Waiting until the binding is created for the workload cluster")
	Eventually(func() bool {
		binding := &addonsv1.ClusterResourceSetBinding{}
		err := input.ClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Name: input.Cluster.Name, Namespace: input.Cluster.Namespace}, binding)
		return err == nil
	}, intervals...).Should(BeTrue())

	fmt.Fprintln(GinkgoWriter, "Waiting until the resource is created in the workload cluster")
	Eventually(func() bool {
		binding := &addonsv1.ClusterResourceSetBinding{}
		Expect(
			input.ClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Name: input.Cluster.Name, Namespace: input.Cluster.Namespace}, binding),
		).To(Succeed())

		for _, resource := range input.ClusterResourceSet.Spec.Resources {
			var configSource client.Object

			switch resource.Kind {
			case string(addonsv1.SecretClusterResourceSetResourceKind):
				configSource = &corev1.Secret{}
			case string(addonsv1.ConfigMapClusterResourceSetResourceKind):
				configSource = &corev1.ConfigMap{}
			}

			if err := input.ClusterProxy.GetClient().Get(
				ctx, types.NamespacedName{Name: resource.Name, Namespace: input.ClusterResourceSet.Namespace}, configSource,
			); err != nil {
				// If the resource is missing, CRS will not requeue but retry at each reconcile,
				// because this is not an error. So, we are only interested in seeing the resources that exist to be applied by CRS.
				continue
			}

			// Check relevant ResourceSetBinding to see if the resource is applied. If no ResourceSetBinding is found for
			// the specified ClusterResourceSet, the resource has not applied.
			resourceSetBinding, found := getResourceSetBindingForClusterResourceSet(
				binding,
				input.ClusterResourceSet,
			)
			if !found || !resourceSetBinding.IsApplied(resource) {
				return false
			}
		}
		return true
	}, intervals...).Should(BeTrue())
}

func getResourceSetBindingForClusterResourceSet(
	clusterResourceSetBinding *addonsv1.ClusterResourceSetBinding,
	clusterResourceSet *addonsv1.ClusterResourceSet,
) (*addonsv1.ResourceSetBinding, bool) {
	for _, binding := range clusterResourceSetBinding.Spec.Bindings {
		if binding.ClusterResourceSetName == clusterResourceSet.Name {
			return binding, true
		}
	}
	return nil, false
}
