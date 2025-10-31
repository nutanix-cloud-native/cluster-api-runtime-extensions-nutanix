//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/framework"
)

var _ = Describe("Self-hosted", Serial, func() {
	for _, provider := range []string{"Docker", "Nutanix"} {
		// Add any provider specific decorators here.
		// Docker and Nutanix require Serial decorator to ensure the machine running the e2e tests
		// doesn't have resources exhausted and lead to flaky tests.
		// This prevents parallel execution which can cause IP address pool exhaustion,
		// VM capacity limits, and Prism Central API rate limiting.
		var providerSpecificDecorators []interface{}
		if provider == "Docker" || provider == "Nutanix" {
			providerSpecificDecorators = append(providerSpecificDecorators, Serial)
		}
		Context(provider, Label("provider:"+provider), providerSpecificDecorators, func() {
			lowercaseProvider := strings.ToLower(provider)
			for _, cniProvider := range []string{"Cilium"} { // TODO: Reenable Calico tests later once we fix flakiness.
				Context(cniProvider, Label("cni:"+cniProvider), func() {
					for _, addonStrategy := range []string{"HelmAddon", "ClusterResourceSet"} {
						Context(addonStrategy, Label("addonStrategy:"+addonStrategy), func() {
							strategy := ""
							switch addonStrategy {
							case "HelmAddon":
								strategy = "helm-addon"
							case "ClusterResourceSet":
								strategy = "crs"
							default:
								Fail("unknown addon strategy: " + addonStrategy)
							}
							flavor := fmt.Sprintf(
								"topology-%s-%s",
								strings.ToLower(cniProvider),
								strategy,
							)
							Context(
								flavor,
								func() {
									framework.SelfHostedSpec(
										ctx,
										func() framework.SelfHostedSpecInput {
											clusterNamePrefix := "self-hosted-"
											// To be able to test the self-hosted cluster with long name, we need to set the
											// maxClusterNameLength to 63 which is the maximum length of a cluster name.
											maxClusterNameLength := 63
											// However, if the provider is Docker, we need to reduce the maxClusterNameLength
											// because CAPI adds multiple random suffixes to the cluster name which are used in
											// Docker cluster, and then in MachineDeployment, and finally in the machine names,
											// resulting in 23 extra characters added to the cluster name for the actual worker node
											// machine names. These machine names are used for Docker container names. The Docker
											// image used by KinD have a maximum hostname length of 64 characters. This can be
											// determined by running `getconf HOST_NAME_MAX` in a Docker container and is different
											// from the host which generally allows 255 characters nowadays.
											// Any longer than this prevents the container from starting, returning an error such
											// as `error during container init: sethostname: invalid argument: unknown`.
											// Therefore we reduce the maximum cluster to 64 (max hostname length) - 23 (random suffixes).
											if lowercaseProvider == "docker" {
												maxClusterNameLength = 64 - 23
											}

											return framework.SelfHostedSpecInput{
												E2EConfig:              e2eConfig,
												ClusterctlConfigPath:   clusterctlConfigPath,
												BootstrapClusterProxy:  bootstrapClusterProxy,
												ArtifactFolder:         artifactFolder,
												SkipCleanup:            skipCleanup,
												Flavor:                 flavor,
												InfrastructureProvider: ptr.To(lowercaseProvider),
												ClusterName: ptr.To(clusterNamePrefix +
													util.RandomString(
														maxClusterNameLength-len(clusterNamePrefix),
													)),
												PostClusterMoved: func(proxy capiframework.ClusterProxy, cluster *clusterv1.Cluster) {
													By(
														"Waiting for all requested addons to be ready in workload cluster",
													)
													workloadCluster := capiframework.GetClusterByName(
														ctx,
														capiframework.GetClusterByNameInput{
															Namespace: cluster.GetNamespace(),
															Name:      cluster.GetName(),
															Getter:    proxy.GetClient(),
														},
													)
													Expect(
														workloadCluster.Spec.Topology,
													).ToNot(BeNil())
													clusterVars := variables.ClusterVariablesToVariablesMap(
														workloadCluster.Spec.Topology.Variables,
													)
													addonsConfig, err := variables.Get[apivariables.Addons](
														clusterVars,
														v1alpha1.ClusterConfigVariableName,
														"addons",
													)
													Expect(err).ToNot(HaveOccurred())
													WaitForAddonsToBeReadyInWorkloadCluster(
														ctx,
														WaitForAddonsToBeReadyInWorkloadClusterInput{
															AddonsConfig:    addonsConfig,
															ClusterProxy:    proxy,
															WorkloadCluster: workloadCluster,
															DeploymentIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-deployment",
															),
															DaemonSetIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-daemonset",
															),
															StatefulSetIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-statefulset",
															),
															HelmReleaseIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-helmrelease",
															),
															ClusterResourceSetIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-clusterresourceset",
															),
															ResourceIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-resource",
															),
														},
													)

													WaitForCoreDNSImageVersion(
														ctx,
														WaitForDNSUpgradeInput{
															WorkloadCluster: workloadCluster,
															ClusterProxy:    proxy,
															DeploymentIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-deployment",
															),
														},
													)
													WaitForCoreDNSToBeReadyInWorkloadCluster(
														ctx,
														WaitForCoreDNSToBeReadyInWorkloadClusterInput{
															WorkloadCluster: workloadCluster,
															ClusterProxy:    proxy,
															DeploymentIntervals: e2eConfig.GetIntervals(
																flavor,
																"wait-deployment",
															),
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
												},
											}
										},
									)
								},
							)
						})
					}
				})
			}
		})
	}
})
