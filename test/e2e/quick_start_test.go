//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	"sigs.k8s.io/cluster-api/util"

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
											testE2EConfig.Variables[capie2e.KubernetesVersion] = testE2EConfig.MustGetVariable(
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
												testE2EConfig.MustGetVariable("NUTANIX_SUBNET_NAME"),
												testE2EConfig.MustGetVariable(
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

										clusterNamePrefix := "quick-start-"
										// To be able to test the self-hosted cluster with long name, we need to set the
										// maxClusterNameength to 63 which is the maximum length of a cluster name.
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

										return capie2e.QuickStartSpecInput{
											ClusterName: ptr.To(clusterNamePrefix +
												util.RandomString(
													maxClusterNameLength-len(clusterNamePrefix),
												)),
											E2EConfig:              testE2EConfig,
											ClusterctlConfigPath:   clusterLocalClusterctlConfigPath,
											BootstrapClusterProxy:  bootstrapClusterProxy,
											ArtifactFolder:         artifactFolder,
											SkipCleanup:            skipCleanup,
											Flavor:                 ptr.To(flavor),
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
														KubernetesVersion: testE2EConfig.MustGetVariable(
															capie2e.KubernetesVersion,
														),
														Count: nodeCount,
														WaitForNodesReady: testE2EConfig.GetIntervals(
															flavor,
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
															flavor,
															"wait-deployment",
														),
														DaemonSetIntervals: testE2EConfig.GetIntervals(
															flavor,
															"wait-daemonset",
														),
														StatefulSetIntervals: testE2EConfig.GetIntervals(
															flavor,
															"wait-statefulset",
														),
														HelmReleaseIntervals: testE2EConfig.GetIntervals(
															flavor,
															"wait-helmrelease",
														),
														ClusterResourceSetIntervals: testE2EConfig.GetIntervals(
															flavor,
															"wait-clusterresourceset",
														),
														ResourceIntervals: testE2EConfig.GetIntervals(
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
														DeploymentIntervals: testE2EConfig.GetIntervals(
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
														DeploymentIntervals: testE2EConfig.GetIntervals(
															flavor,
															"wait-deployment",
														),
													},
												)

												if os.Getenv("RUN_CIS_BENCHMARK") == "true" {
													By("Running CIS benchmark against workload cluster")

													trivyCmd := exec.Command( //nolint:gosec // Only used for testing so safe here.
														"trivy",
														"k8s",
														"--compliance=k8s-cis-1.23",
														"--disable-node-collector",
														"--report=summary",
														fmt.Sprintf(
															"--output=%s",
															filepath.Join(
																os.Getenv("GIT_REPO_ROOT"),
																"cis-benchmark-report.txt",
															),
														),
														fmt.Sprintf(
															"--kubeconfig=%s",
															workloadProxy.GetKubeconfigPath(),
														),
													)

													trivyCmd.Stdout = GinkgoWriter
													trivyCmd.Stderr = GinkgoWriter

													Expect(trivyCmd.Run()).To(Succeed(), "CIS benchmark failed")
												}
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
