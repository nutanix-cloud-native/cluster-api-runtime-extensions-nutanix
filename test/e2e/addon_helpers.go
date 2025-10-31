//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

type WaitForAddonsToBeReadyInWorkloadClusterInput struct {
	AddonsConfig                apivariables.Addons
	WorkloadCluster             *clusterv1.Cluster
	InfrastructureProvider      string
	ClusterProxy                framework.ClusterProxy
	DeploymentIntervals         []interface{}
	DaemonSetIntervals          []interface{}
	StatefulSetIntervals        []interface{}
	HelmReleaseIntervals        []interface{}
	ClusterResourceSetIntervals []interface{}
	ResourceIntervals           []interface{}
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

	WaitForClusterAutoscalerToBeReadyForWorkloadCluster(
		ctx,
		WaitForClusterAutoscalerToBeReadyForWorkloadClusterInput{
			ClusterAutoscaler:           input.AddonsConfig.ClusterAutoscaler,
			WorkloadCluster:             input.WorkloadCluster,
			ClusterProxy:                input.ClusterProxy,
			DeploymentIntervals:         input.DeploymentIntervals,
			DaemonSetIntervals:          input.DaemonSetIntervals,
			HelmReleaseIntervals:        input.HelmReleaseIntervals,
			ClusterResourceSetIntervals: input.ClusterResourceSetIntervals,
		},
	)

	WaitForCCMToBeReadyInWorkloadCluster(
		ctx,
		WaitForCCMToBeReadyInWorkloadClusterInput{
			CCM:                         input.AddonsConfig.CCM,
			InfrastructureProvider:      input.InfrastructureProvider,
			WorkloadCluster:             input.WorkloadCluster,
			ClusterProxy:                input.ClusterProxy,
			DeploymentIntervals:         input.DeploymentIntervals,
			DaemonSetIntervals:          input.DaemonSetIntervals,
			HelmReleaseIntervals:        input.HelmReleaseIntervals,
			ClusterResourceSetIntervals: input.ClusterResourceSetIntervals,
		},
	)

	WaitForCSIToBeReadyInWorkloadCluster(
		ctx,
		WaitForCSIToBeReadyInWorkloadClusterInput{
			CSI:                         input.AddonsConfig.CSI,
			WorkloadCluster:             input.WorkloadCluster,
			ClusterProxy:                input.ClusterProxy,
			DeploymentIntervals:         input.DeploymentIntervals,
			DaemonSetIntervals:          input.DaemonSetIntervals,
			HelmReleaseIntervals:        input.HelmReleaseIntervals,
			ClusterResourceSetIntervals: input.ClusterResourceSetIntervals,
		},
	)

	WaitForCOSIControllerToBeReadyInWorkloadCluster(
		ctx,
		WaitForCOSIControllerToBeReadyInWorkloadClusterInput{
			COSI:                 input.AddonsConfig.COSI,
			WorkloadCluster:      input.WorkloadCluster,
			ClusterProxy:         input.ClusterProxy,
			DeploymentIntervals:  input.DeploymentIntervals,
			HelmReleaseIntervals: input.HelmReleaseIntervals,
		},
	)

	WaitForServiceLoadBalancerToBeReadyInWorkloadCluster(
		ctx,
		WaitForServiceLoadBalancerToBeReadyInWorkloadClusterInput{
			ServiceLoadBalancer:  input.AddonsConfig.ServiceLoadBalancer,
			WorkloadCluster:      input.WorkloadCluster,
			ClusterProxy:         input.ClusterProxy,
			DeploymentIntervals:  input.DeploymentIntervals,
			DaemonSetIntervals:   input.DaemonSetIntervals,
			HelmReleaseIntervals: input.HelmReleaseIntervals,
			ResourceIntervals:    input.ResourceIntervals,
		},
	)

	WaitForRegistryAddonToBeReadyInWorkloadCluster(
		ctx,
		WaitForRegistryAddonToBeReadyInWorkloadClusterInput{
			Registry:             input.AddonsConfig.Registry,
			WorkloadCluster:      input.WorkloadCluster,
			ClusterProxy:         input.ClusterProxy,
			StatefulSetIntervals: input.StatefulSetIntervals,
		},
	)

	WaitForIngressToBeReadyInWorkloadCluster(
		ctx,
		WaitForIngressToBeReadyInWorkloadClusterInput{
			Ingress:              input.AddonsConfig.Ingress,
			WorkloadCluster:      input.WorkloadCluster,
			ClusterProxy:         input.ClusterProxy,
			DeploymentIntervals:  input.DeploymentIntervals,
			HelmReleaseIntervals: input.HelmReleaseIntervals,
		},
	)

	WaitForKonnectorAgentToBeReadyInWorkloadCluster(
		ctx,
		WaitForKonnectorAgentToBeReadyInWorkloadClusterInput{
			KonnectorAgent:       input.AddonsConfig.NutanixKonnectorAgent,
			WorkloadCluster:      input.WorkloadCluster,
			ClusterProxy:         input.ClusterProxy,
			DeploymentIntervals:  input.DeploymentIntervals,
			HelmReleaseIntervals: input.HelmReleaseIntervals,
		},
	)
}
