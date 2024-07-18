//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/framework/nutanix"
)

var _ = Describe("Quick start", func() {
	for _, provider := range []string{"Docker", "AWS", "Nutanix"} {
		// Add any provider specific decorators here.
		// Currently, only Docker requires Serial decorator to ensure the machine running the Docker e2e tests
		// doesn't have resources exhausted and lead to flaky tests.
		// Other provider tests will run in parallel.
		var providerSpecificDecorators []interface{}
		if provider == "Docker" {
			providerSpecificDecorators = append(providerSpecificDecorators, Serial)
		}

		Context(provider, Label("provider:"+provider), providerSpecificDecorators, func() {
			lowercaseProvider := strings.ToLower(provider)
			for _, cniProvider := range []string{"Cilium", "Calico"} {
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
							flavour := fmt.Sprintf(
								"topology-%s-%s",
								strings.ToLower(cniProvider),
								strategy,
							)
							Context(
								flavour,
								func() {
									var (
										testE2EConfig                    *clusterctl.E2EConfig
										clusterLocalClusterctlConfigPath string
									)

									BeforeEach(func() {
										testE2EConfig = e2eConfig.DeepCopy()

										// Check if a provider-specific Kubernetes version is set in the environment and use that. This allows
										// for testing against different Kubernetes versions, as some providers (e.g. Docker) have machine images
										// available that are not available in other providers.
										// This version can be specified in `test/e2e/config/caren.yaml` with a variable named
										// `KUBERNETES_VERSION_<PROVIDER>`, where `<PROVIDER>` is the uppercase provider name, e.g.
										// `KUBERNETES_VERSION_DOCKER: v1.29.5`.
										varName := capie2e.KubernetesVersion + "_" + strings.ToUpper(
											lowercaseProvider,
										)
										if testE2EConfig.HasVariable(varName) {
											testE2EConfig.Variables[capie2e.KubernetesVersion] = testE2EConfig.GetVariable(
												varName,
											)
										}

										// For Nutanix provider, reserve an IP address for the workload cluster control plane endpoint -
										// remember to unreserve it!
										if provider == "Nutanix" {
											By(
												"Reserving an IP address for the workload cluster control plane endpoint",
											)
											nutanixClient, err := nutanix.NewV4Client(
												nutanix.CredentialsFromCAPIE2EConfig(testE2EConfig),
											)
											Expect(err).ToNot(HaveOccurred())

											controlPlaneEndpointIP, unreserveControlPlaneEndpointIP, err := nutanix.ReserveIP(
												testE2EConfig.GetVariable("NUTANIX_SUBNET_NAME"),
												testE2EConfig.GetVariable(
													"NUTANIX_PRISM_ELEMENT_CLUSTER_NAME",
												),
												nutanixClient,
											)
											Expect(err).ToNot(HaveOccurred())
											DeferCleanup(unreserveControlPlaneEndpointIP)
											testE2EConfig.Variables["CONTROL_PLANE_ENDPOINT_IP"] = controlPlaneEndpointIP
										}

										clusterLocalTempDir, err := os.MkdirTemp("", "clusterctl-")
										Expect(err).ToNot(HaveOccurred())
										DeferCleanup(func() {
											Expect(os.RemoveAll(clusterLocalTempDir)).To(Succeed())
										})
										clusterLocalClusterctlConfigPath = createClusterctlLocalRepository(
											testE2EConfig,
											clusterLocalTempDir,
										)
									})

									capie2e.QuickStartSpec(ctx, func() capie2e.QuickStartSpecInput {
										if !slices.Contains(
											e2eConfig.InfrastructureProviders(),
											lowercaseProvider,
										) {
											Fail(fmt.Sprintf(
												"provider %s is not enabled - check environment setup for provider specific requirements",
												lowercaseProvider,
											))
										}

										return capie2e.QuickStartSpecInput{
											E2EConfig:              testE2EConfig,
											ClusterctlConfigPath:   clusterLocalClusterctlConfigPath,
											BootstrapClusterProxy:  bootstrapClusterProxy,
											ArtifactFolder:         artifactFolder,
											SkipCleanup:            skipCleanup,
											Flavor:                 ptr.To(flavour),
											InfrastructureProvider: ptr.To(lowercaseProvider),
											PostMachinesProvisioned: func(proxy capiframework.ClusterProxy, namespace, clusterName string) {
												capiframework.AssertOwnerReferences(
													namespace,
													proxy.GetKubeconfigPath(),
													filterClusterObjectsWithNameFilterIgnoreClusterAutoscaler(
														clusterName,
													),
													capiframework.CoreOwnerReferenceAssertion,
													capiframework.DockerInfraOwnerReferenceAssertions,
													capiframework.KubeadmBootstrapOwnerReferenceAssertions,
													capiframework.KubeadmControlPlaneOwnerReferenceAssertions,
													AWSInfraOwnerReferenceAssertions,
													NutanixInfraOwnerReferenceAssertions,
													AddonReferenceAssertions,
													KubernetesReferenceAssertions,
												)

												workloadCluster := capiframework.GetClusterByName(
													ctx,
													capiframework.GetClusterByNameInput{
														Namespace: namespace,
														Name:      clusterName,
														Getter:    proxy.GetClient(),
													},
												)
												Expect(workloadCluster.Spec.Topology).ToNot(BeNil())

												By("Waiting until nodes are ready")
												workloadProxy := proxy.GetWorkloadCluster(
													ctx,
													namespace,
													clusterName,
												)
												workloadClient := workloadProxy.GetClient()

												nodeCount := int(
													ptr.Deref(
														workloadCluster.Spec.Topology.ControlPlane.Replicas,
														0,
													),
												) +
													lo.Reduce(
														workloadCluster.Spec.Topology.Workers.MachineDeployments,
														func(agg int, md clusterv1.MachineDeploymentTopology, _ int) int {
															switch {
															case md.Replicas != nil:
																return agg + int(
																	ptr.Deref(md.Replicas, 0),
																)
															case md.Metadata.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"] != "":
																minSize, err := strconv.Atoi(
																	md.Metadata.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"],
																)
																Expect(err).ToNot(HaveOccurred())
																return agg + minSize
															default:
																return agg
															}
														},
														0,
													)

												capiframework.WaitForNodesReady(
													ctx,
													capiframework.WaitForNodesReadyInput{
														Lister: workloadClient,
														KubernetesVersion: testE2EConfig.GetVariable(
															capie2e.KubernetesVersion,
														),
														Count: nodeCount,
														WaitForNodesReady: testE2EConfig.GetIntervals(
															flavour,
															"wait-nodes-ready",
														),
													},
												)

												By(
													"Waiting for all requested addons to be ready in workload cluster",
												)
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
														AddonsConfig:           addonsConfig,
														ClusterProxy:           proxy,
														WorkloadCluster:        workloadCluster,
														InfrastructureProvider: lowercaseProvider,
														DeploymentIntervals: testE2EConfig.GetIntervals(
															flavour,
															"wait-deployment",
														),
														DaemonSetIntervals: testE2EConfig.GetIntervals(
															flavour,
															"wait-daemonset",
														),
														HelmReleaseIntervals: testE2EConfig.GetIntervals(
															flavour,
															"wait-helmrelease",
														),
														ClusterResourceSetIntervals: testE2EConfig.GetIntervals(
															flavour,
															"wait-clusterresourceset",
														),
														ResourceIntervals: testE2EConfig.GetIntervals(
															flavour,
															"wait-resource",
														),
													},
												)

												WaitForCoreDNSToBeReadyInWorkloadCluster(
													ctx,
													WaitForCoreDNSToBeReadyInWorkloadClusterInput{
														WorkloadCluster: workloadCluster,
														ClusterProxy:    proxy,
														DeploymentIntervals: testE2EConfig.GetIntervals(
															flavour,
															"wait-deployment",
														),
													},
												)
											},
										}
									})
								},
							)
						})
					}
				})
			}
		})
	}
})
