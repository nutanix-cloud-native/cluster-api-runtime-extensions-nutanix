//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

type WaitForAddonsToBeReadyInWorkloadClusterInput struct {
	AddonsConfig                v1alpha1.Addons
	WorkloadCluster             *capiv1.Cluster
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
}

func WaitForAddonsToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForAddonsToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	WaitForCNIToBeReadyInWorkloadCluster(
		ctx,
		WaitForCNIToBeReadyInWorkloadClusterInput{
			CNI:                         input.AddonsConfig.CNI,
			WorkloadCluster:             input.WorkloadCluster,
			ClusterProxy:                input.ClusterProxy,
			DeploymentIntervals:         input.DeploymentIntervals,
			DaemonSetIntervals:          input.DaemonSetIntervals,
			HelmReleaseIntervals:        input.HelmReleaseIntervals,
			ClusterResourceSetIntervals: input.ClusterResourceSetIntervals,
		},
	)

	WaitForNFDToBeReadyInWorkloadCluster(
		ctx,
		WaitForNFDToBeReadyInWorkloadClusterInput{
			NFD:                         input.AddonsConfig.NFD,
			WorkloadCluster:             input.WorkloadCluster,
			ClusterProxy:                input.ClusterProxy,
			DeploymentIntervals:         input.DeploymentIntervals,
			DaemonSetIntervals:          input.DaemonSetIntervals,
			HelmReleaseIntervals:        input.HelmReleaseIntervals,
			ClusterResourceSetIntervals: input.ClusterResourceSetIntervals,
		},
	)
}
