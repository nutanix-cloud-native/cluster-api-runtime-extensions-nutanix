//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

// autoscalerMinSizeAnnotation marks the minimum size of a MachineDeployment managed by the
// cluster autoscaler, which is used in place of an explicit replica count.
const autoscalerMinSizeAnnotation = "cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"

// nutanixProvider is the original-cased name of the Nutanix infrastructure provider.
const (
	nutanixProvider = "Nutanix"
	// nutanixInfraProviderName is the lower-case infrastructure provider name used in templates and variables.
	nutanixInfraProviderName = "nutanix"

	yamlKeyName  = "name"
	yamlKeyValue = "value"
)

// VerifyProvisionedClusterInput is the input for VerifyProvisionedCluster.
type VerifyProvisionedClusterInput struct {
	ClusterProxy capiframework.ClusterProxy
	Namespace    string
	ClusterName  string
	// Provider is the original-cased infrastructure provider name, e.g. "Nutanix".
	Provider string
	// Flavor is the cluster template flavor, used to look up per-flavor wait intervals.
	Flavor    string
	E2EConfig *clusterctl.E2EConfig
}

// VerifyProvisionedCluster runs the assertions shared by every CAREN quick-start style spec:
// owner references, node readiness, readiness of every requested addon (CNI, CCM, CSI, COSI,
// service load balancer, ...), CoreDNS, the registry addon, and - on Nutanix - a working
// LoadBalancer Service. It is intended to be called from PostMachinesProvisioned, and returns
// the workload Cluster so callers can layer on flavor-specific assertions.
func VerifyProvisionedCluster(
	ctx context.Context,
	input VerifyProvisionedClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) *clusterv1.Cluster {
	lowercaseProvider := strings.ToLower(input.Provider)
	proxy := input.ClusterProxy

	capiframework.AssertOwnerReferences(
		input.Namespace,
		proxy.GetKubeconfigPath(),
		filterClusterObjectsWithNameFilterIgnoreClusterAutoscaler(input.ClusterName),
		capiframework.CoreOwnerReferenceAssertion,
		capiframework.DevInfraOwnerReferenceAssertions(true),
		capiframework.KubeadmBootstrapOwnerReferenceAssertions,
		capiframework.KubeadmControlPlaneOwnerReferenceAssertions(true),
		AWSInfraOwnerReferenceAssertions,
		NutanixInfraOwnerReferenceAssertions,
		AddonReferenceAssertions,
		KubernetesReferenceAssertions,
	)

	workloadCluster := capiframework.GetClusterByName(
		ctx,
		capiframework.GetClusterByNameInput{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
			Getter:    proxy.GetClient(),
		},
	)
	Expect(workloadCluster.Spec.Topology).ToNot(BeNil())

	By("Waiting until nodes are ready")
	workloadProxy := proxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)

	capiframework.WaitForNodesReady(
		ctx,
		capiframework.WaitForNodesReadyInput{
			Lister:            workloadProxy.GetClient(),
			KubernetesVersion: input.E2EConfig.MustGetVariable(capie2e.KubernetesVersion),
			Count:             expectedNodeCount(workloadCluster),
			WaitForNodesReady: input.E2EConfig.GetIntervals(input.Flavor, "wait-nodes-ready"),
		},
	)

	By("Waiting for all requested addons to be ready in workload cluster")
	clusterVars := variables.ClusterVariablesToVariablesMap(workloadCluster.Spec.Topology.Variables)
	addonsConfig, err := variables.Get[apivariables.Addons](
		clusterVars,
		v1alpha1.ClusterConfigVariableName,
		"addons",
	)
	Expect(err).ToNot(HaveOccurred())
	WaitForAddonsToBeReadyInWorkloadCluster(
		ctx,
		WaitForAddonsToBeReadyInWorkloadClusterInput{
			AddonsConfig:                addonsConfig,
			ClusterProxy:                proxy,
			WorkloadCluster:             workloadCluster,
			InfrastructureProvider:      lowercaseProvider,
			DeploymentIntervals:         input.E2EConfig.GetIntervals(input.Flavor, "wait-deployment"),
			DaemonSetIntervals:          input.E2EConfig.GetIntervals(input.Flavor, "wait-daemonset"),
			StatefulSetIntervals:        input.E2EConfig.GetIntervals(input.Flavor, "wait-statefulset"),
			HelmReleaseIntervals:        input.E2EConfig.GetIntervals(input.Flavor, "wait-helmrelease"),
			ClusterResourceSetIntervals: input.E2EConfig.GetIntervals(input.Flavor, "wait-clusterresourceset"),
			ResourceIntervals:           input.E2EConfig.GetIntervals(input.Flavor, "wait-resource"),
		},
	)

	WaitForCoreDNSImageVersion(
		ctx,
		WaitForDNSUpgradeInput{
			WorkloadCluster:     workloadCluster,
			ClusterProxy:        proxy,
			DeploymentIntervals: input.E2EConfig.GetIntervals(input.Flavor, "wait-deployment"),
		},
	)
	WaitForCoreDNSToBeReadyInWorkloadCluster(
		ctx,
		WaitForCoreDNSToBeReadyInWorkloadClusterInput{
			WorkloadCluster:     workloadCluster,
			ClusterProxy:        proxy,
			DeploymentIntervals: input.E2EConfig.GetIntervals(input.Flavor, "wait-deployment"),
		},
	)

	EnsureClusterCAForRegistryAddon(
		ctx,
		EnsureClusterCAForRegistryAddonInput{
			Registry:        addonsConfig.Registry,
			WorkloadCluster: workloadCluster,
			ClusterProxy:    proxy,
		},
	)
	EnsureAntiAffnityForRegistryAddon(
		ctx,
		EnsureAntiAffnityForRegistryAddonInput{
			Registry:        addonsConfig.Registry,
			WorkloadCluster: workloadCluster,
			ClusterProxy:    proxy,
		},
	)

	// TODO: Test for other providers.
	if input.Provider == nutanixProvider {
		AssertClusterResolvedNutanixProject(workloadCluster)

		EnsureLoadBalancerService(
			ctx,
			EnsureLoadBalancerServiceInput{
				WorkloadCluster:  workloadCluster,
				ClusterProxy:     proxy,
				ServiceIntervals: input.E2EConfig.GetIntervals(input.Flavor, "wait-service"),
			},
		)
	}

	return workloadCluster
}

// expectedNodeCount returns the number of nodes the cluster topology asks for, treating a
// MachineDeployment managed by the cluster autoscaler as its configured minimum size.
func expectedNodeCount(cluster *clusterv1.Cluster) int {
	return int(ptr.Deref(cluster.Spec.Topology.ControlPlane.Replicas, 0)) +
		lo.Reduce(
			cluster.Spec.Topology.Workers.MachineDeployments,
			func(agg int, md clusterv1.MachineDeploymentTopology, _ int) int {
				switch {
				case md.Replicas != nil:
					return agg + int(ptr.Deref(md.Replicas, 0))
				case md.Metadata.Annotations[autoscalerMinSizeAnnotation] != "":
					minSize, err := strconv.Atoi(md.Metadata.Annotations[autoscalerMinSizeAnnotation])
					Expect(err).ToNot(HaveOccurred())
					return agg + minSize
				default:
					return agg
				}
			},
			0,
		)
}
