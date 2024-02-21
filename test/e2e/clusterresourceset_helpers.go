//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	addonsv1 "sigs.k8s.io/cluster-api/exp/addons/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
)

type waitForClusterResourceSetToApplyResourcesInClusterInput struct {
	name         string
	clusterProxy framework.ClusterProxy
	cluster      *capiv1.Cluster
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

	framework.WaitForClusterResourceSetToApplyResources(
		ctx,
		framework.WaitForClusterResourceSetToApplyResourcesInput{
			ClusterProxy:       input.clusterProxy,
			Cluster:            input.cluster,
			ClusterResourceSet: crs,
		},
		input.intervals...,
	)
}
