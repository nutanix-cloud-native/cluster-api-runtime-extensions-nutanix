//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/helpers/oidc"
)

type providerConfiguration struct {
	cniProviders map[string][]string
}

var providerConfigurations = map[string]providerConfiguration{
	"Docker": {
		cniProviders: map[string][]string{
			// TODO: Reenable Calico tests later once we fix flakiness.
			//"Calico": {"HelmAddon", "ClusterResourceSet"},
			"Cilium": {"HelmAddon", "ClusterResourceSet"},
		},
	},
	"Nutanix": {
		cniProviders: map[string][]string{
			"Cilium": {"HelmAddon", "ClusterResourceSet"},
			// TODO: Reenable Calico tests later once we fix flakiness.
			// "Calico": {"HelmAddon", "ClusterResourceSet"},
			"Flow": {"HelmAddon"},
		},
	},
	"AWS": {
		cniProviders: map[string][]string{
			"Cilium": {"HelmAddon", "ClusterResourceSet"},
			// TODO: Reenable Calico tests later once we fix flakiness.
			// "Calico": {"HelmAddon", "ClusterResourceSet"},
		},
	},
}

var _ = Describe("Quick start", func() {
	for provider, providerCfg := range providerConfigurations {
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
			for cniProvider, addonStrategies := range providerCfg.cniProviders {
				Context(cniProvider, Label("cni:"+cniProvider), func() {
					BeforeEach(func() {
						// For Flow CNI, we need to set the Docker Hub username and password
						// because the Flow CNI images require a Docker account to pull the images.
						if cniProvider == "Flow" {
							if e2eConfig.GetVariableOrEmpty("NUTANIX_FLOW_DOCKER_HUB_USERNAME") == "" ||
								e2eConfig.GetVariableOrEmpty("NUTANIX_FLOW_DOCKER_HUB_TOKEN") == "" {
								Skip(
									"Both NUTANIX_FLOW_DOCKER_HUB_USERNAME and NUTANIX_FLOW_DOCKER_HUB_TOKEN must be set",
								)
							}
						}
					})

					for _, addonStrategy := range addonStrategies {
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

							flavors := []string{}
							flavors = append(
								flavors,
								fmt.Sprintf("topology-%s-%s", strings.ToLower(cniProvider), strategy),
							)
							if provider == "Nutanix" && cniProvider == "Cilium" {
								flavors = append(
									flavors,
									fmt.Sprintf(
										"topology-with-failuredomains-%s-%s",
										strings.ToLower(cniProvider),
										strategy,
									),
								)
							}
							for _, flavor := range flavors {
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
											applyProviderKubernetesVersionOverride(testE2EConfig, lowercaseProvider)

											// For Nutanix provider, reserve an IP address for the workload cluster:
											// 1. control plane endpoint
											// 2. service load balancer
											// Remember to unreserve it after the test!
											if provider == "Nutanix" {
												reserveNutanixIPsForCluster(testE2EConfig)
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
															func(agg int, md clusterv1beta2.MachineDeploymentTopology, _ int) int {
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
													if provider == "Nutanix" {
														EnsureLoadBalancerService(
															ctx,
															EnsureLoadBalancerServiceInput{
																WorkloadCluster: workloadCluster,
																ClusterProxy:    proxy,
																ServiceIntervals: testE2EConfig.GetIntervals(
																	flavor,
																	"wait-service",
																),
															},
														)
													}

													// Authentication e2e: extend quick-start with in-cluster mock OIDC
													// to avoid spinning a dedicated Cluster just for auth. Runs for
													// the first flavor per provider (provider-agnostic) as part of
													// normal quick-start, without extra provider label.
													if flavor == fmt.Sprintf(
														"topology-%s-%s",
														strings.ToLower(cniProvider),
														strategy,
													) {
														By(
															"Authentication with mock OIDC (provider-agnostic, in-cluster)",
														)
														workloadProxy := proxy.GetWorkloadCluster(
															ctx,
															namespace,
															clusterName,
														)
														workloadClient := workloadProxy.GetClient()
														mock, err := oidc.SetupMockOIDC(ctx, oidc.SetupInput{
															WorkloadClient: workloadClient,
															Namespace:      "oidc-mock",
														})
														Expect(err).ToNot(HaveOccurred(), "setting up mock OIDC")
														By(fmt.Sprintf("Mock OIDC issuer: %s", mock.IssuerURL))

														Eventually(func() bool {
															dep := &appsv1.Deployment{}
															if err := workloadClient.Get(
																ctx,
																client.ObjectKey{
																	Namespace: "oidc-mock",
																	Name:      "mock-oidc",
																},
																dep,
															); err != nil {
																return false
															}
															for _, c := range dep.Status.Conditions {
																if c.Type == appsv1.DeploymentAvailable &&
																	c.Status == corev1.ConditionTrue {
																	return true
																}
															}
															return false
														}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "mock OIDC deployment not available")

														By("Patching Cluster with authentication configuration")
														bootstrapClient := proxy.GetClient()
														cluster := &clusterv1beta2.Cluster{}
														Expect(
															bootstrapClient.Get(
																ctx,
																client.ObjectKey{
																	Namespace: namespace,
																	Name:      clusterName,
																},
																cluster,
															),
														).To(Succeed())
														Expect(cluster.Spec.Topology).ToNot(BeNil())
														auth := v1alpha1.Authentication{
															JWT: []v1alpha1.JWTAuthenticator{
																{
																	Issuer: v1alpha1.Issuer{
																		URL:                  mock.IssuerURL,
																		DiscoveryURL:         mock.DiscoveryURL,
																		Audiences:            []string{"e2e-aud"},
																		AudienceMatchPolicy:  v1alpha1.AudienceMatchPolicyMatchAny,
																		CertificateAuthority: mock.CAPEM,
																	},
																	ClaimMappings: v1alpha1.ClaimMappings{
																		Username: v1alpha1.PrefixedClaimOrExpression{
																			Claim: "email",
																		},
																		Groups: &v1alpha1.PrefixedClaimOrExpression{
																			Claim: "groups",
																		},
																		Extra: []v1alpha1.ExtraMapping{
																			{
																				Key:             "example.org/foo",
																				ValueExpression: "claims.foo",
																			},
																		},
																	},
																	ClaimValidationRules: []v1alpha1.ClaimValidationRule{
																		{Claim: "hd", RequiredValue: "example.com"},
																	},
																	UserValidationRules: []v1alpha1.UserValidationRule{
																		{
																			Expression: "!user.username.startsWith('system:')",
																			Message:    "system: prefix reserved",
																		},
																	},
																},
															},
														}
														cluster.Spec.Topology.Variables = upsertAuthenticationVariable(
															cluster.Spec.Topology.Variables,
															auth,
														)
														Expect(bootstrapClient.Update(ctx, cluster)).To(Succeed())

														By("Waiting for Cluster to be Ready after authentication patch")
														Eventually(func() bool {
															c := &clusterv1beta2.Cluster{}
															if err := bootstrapClient.Get(
																ctx,
																client.ObjectKey{
																	Namespace: namespace,
																	Name:      clusterName,
																},
																c,
															); err != nil {
																return false
															}
															for _, cond := range c.Status.Conditions {
																if cond.Type == clusterv1beta2.ReadyCondition &&
																	cond.Status == metav1.ConditionTrue {
																	return true
																}
															}
															return false
														}, 10*time.Minute, 10*time.Second).Should(BeTrue(), "Cluster not Ready after authentication patch")

														By("Verifying admin kubeconfig still works after auth")
														pods := &corev1.PodList{}
														Expect(
															workloadClient.List(
																ctx,
																pods,
																client.InNamespace("kube-system"),
															),
														).To(Succeed())

														By("Validating valid JWT is accepted")
														validClaims := map[string]interface{}{
															"email":  "test@example.com",
															"groups": []string{"e2e-group"},
															"hd":     "example.com",
															"foo":    "bar",
															"aud":    "e2e-aud",
															"sub":    "123",
														}
														validToken, err := mock.MintJWT(validClaims)
														Expect(err).ToNot(HaveOccurred())
														Expect(
															validateTokenAuth(ctx, workloadProxy, validToken, true),
														).To(Succeed())

														By("Validating invalid JWTs are rejected")
														invalidCases := []map[string]interface{}{
															{
																"email":  "test@example.com",
																"groups": []string{"e2e-group"},
																"hd":     "wrong.com",
																"aud":    "e2e-aud",
															},
															{
																"email": "test@example.com",
																"hd":    "example.com",
																"aud":   "wrong-aud",
															},
															{
																"email": "test@example.com",
																"hd":    "example.com",
																"aud":   "e2e-aud",
																"exp":   time.Now().Add(-time.Hour).Unix(),
															},
														}
														for _, claims := range invalidCases {
															token, err := mock.MintJWT(claims)
															Expect(err).ToNot(HaveOccurred())
															Expect(
																validateTokenAuth(ctx, workloadProxy, token, false),
															).To(Succeed())
														}

														By(
															"Validating system: prefix is rejected via userValidationRule",
														)
														sysClaims := map[string]interface{}{
															"email": "system:evil@example.com",
															"hd":    "example.com",
															"aud":   "e2e-aud",
														}
														sysToken, err := mock.MintJWT(sysClaims)
														Expect(err).ToNot(HaveOccurred())
														Expect(
															validateTokenAuth(ctx, workloadProxy, sysToken, false),
														).To(Succeed())
													}
												},
											}
										})
									},
								)
							}
						})
					}
				})
			}
		})
	}
})

func upsertAuthenticationVariable(
	vars []clusterv1beta2.ClusterVariable,
	auth v1alpha1.Authentication,
) []clusterv1beta2.ClusterVariable {
	authMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&auth)
	Expect(err).ToNot(HaveOccurred())
	for i, v := range vars {
		if v.Name == "clusterConfig" {
			existing := map[string]interface{}{}
			if len(v.Value.Raw) > 0 {
				Expect(json.Unmarshal(v.Value.Raw, &existing)).To(Succeed())
			}
			existing["authentication"] = authMap
			raw, err := json.Marshal(existing)
			Expect(err).ToNot(HaveOccurred())
			vars[i].Value.Raw = raw
			return vars
		}
	}
	payload := map[string]interface{}{"authentication": authMap}
	raw, err := json.Marshal(payload)
	Expect(err).ToNot(HaveOccurred())
	return append(vars, clusterv1beta2.ClusterVariable{
		Name:  "clusterConfig",
		Value: apiextensionsv1.JSON{Raw: raw},
	})
}

func validateTokenAuth(ctx context.Context, proxy capiframework.ClusterProxy, token string, shouldSucceed bool) error {
	restConfig := rest.CopyConfig(proxy.GetRESTConfig())
	restConfig.BearerToken = token
	restConfig.BearerTokenFile = ""
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	c, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return err
	}
	pods := &corev1.PodList{}
	err = c.List(ctx, pods, client.InNamespace("default"))
	if shouldSucceed {
		if err != nil {
			return fmt.Errorf("expected token to succeed but got error: %w", err)
		}
		return nil
	}
	if err == nil {
		return fmt.Errorf("expected token to be rejected but succeeded")
	}
	if strings.Contains(err.Error(), "Unauthorized") || strings.Contains(err.Error(), "Forbidden") ||
		strings.Contains(err.Error(), "401") ||
		strings.Contains(err.Error(), "403") {
		return nil
	}
	return fmt.Errorf("expected Unauthorized/Forbidden but got: %w", err)
}
