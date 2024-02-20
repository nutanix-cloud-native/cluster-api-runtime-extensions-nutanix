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

func waitForClusterResourceSetToApplyResourcesInCluster(
	ctx context.Context,
	namespace, name string,
	clusterProxy framework.ClusterProxy,
	cluster *capiv1.Cluster,
	intervals ...interface{},
) {
	crs := &addonsv1.ClusterResourceSet{}
	Expect(clusterProxy.GetClient().Get(
		ctx, types.NamespacedName{Name: name, Namespace: namespace}, crs,
	)).To(Succeed())

	framework.WaitForClusterResourceSetToApplyResources(
		ctx,
		framework.WaitForClusterResourceSetToApplyResourcesInput{
			ClusterProxy:       clusterProxy,
			Cluster:            cluster,
			ClusterResourceSet: crs,
		},
		intervals...,
	)
}
