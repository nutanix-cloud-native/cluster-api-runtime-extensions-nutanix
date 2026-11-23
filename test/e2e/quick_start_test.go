//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/framework/oidc"
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
											clusterName                      string
											mockOIDCBundle                   *oidc.Bundle
											saIssuerConfig                   *v1alpha1.ServiceAccountIssuer
										)

										BeforeEach(func() {
											testE2EConfig = e2eConfig.DeepCopy()
											isolateProviderVersions(testE2EConfig, lowercaseProvider)

											// Check if a provider-specific Kubernetes version is set in the environment and use that. This allows
											// for testing against different Kubernetes versions, as some providers (e.g. Docker) have machine images
											// available that are not available in other providers.
											// This version can be specified in `test/e2e/config/caren.yaml` with a variable named
											// `KUBERNETES_VERSION_<PROVIDER>`, where `<PROVIDER>` is the uppercase provider name, e.g.
											// `KUBERNETES_VERSION_DOCKER: v1.29.5`.
											applyProviderKubernetesVersionOverride(testE2EConfig, lowercaseProvider)

											clusterName = generateClusterName("quick-start-", lowercaseProvider)

											// For Nutanix provider, reserve an IP address for the workload cluster:
											// 1. control plane endpoint
											// 2. service load balancer
											// Reservations are only released once the Cluster is confirmed deleted.
											if provider == "Nutanix" {
												reserveNutanixIPsForCluster(testE2EConfig, clusterName)
											}

											clusterLocalTempDir, err := os.MkdirTemp("", "clusterctl-")
											Expect(err).ToNot(HaveOccurred())
											DeferCleanup(func() {
												Expect(os.RemoveAll(clusterLocalTempDir)).To(Succeed())
											})

											// Inject structured authentication and ServiceAccount issuer configuration
											// at cluster creation time on one flavor per provider, avoiding a
											// post-provision control-plane rollout and verifying additive composition.
											mockOIDCBundle = nil
											saIssuerConfig = nil
											if flavor == "topology-cilium-helm-addon" {
												mockOIDCBundle = enableMockOIDCInFlavorTemplate(
													testE2EConfig,
													lowercaseProvider,
													flavor,
													clusterLocalTempDir,
												)
												saIssuerConfig = enableServiceAccountIssuerInFlavorTemplate(
													testE2EConfig,
													lowercaseProvider,
													flavor,
													clusterLocalTempDir,
												)
											}

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
												ClusterName:            ptr.To(clusterName),
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

													if flavor == "topology-cilium-helm-addon" {
														ensureWorkloadClusterAuthentication(
															ctx,
															ensureWorkloadClusterAuthenticationInput{
																BootstrapProxy:  proxy,
																WorkloadProxy:   workloadProxy,
																WorkloadCluster: workloadCluster,
																Namespace:       namespace,
																ClusterName:     clusterName,
																Bundle:          mockOIDCBundle,
																ControlPlaneIntervals: testE2EConfig.GetIntervals(
																	flavor,
																	"wait-control-plane",
																),
																DeploymentIntervals: testE2EConfig.GetIntervals(
																	flavor,
																	"wait-deployment",
																),
															},
														)
														ensureWorkloadClusterServiceAccountIssuer(
															ctx,
															ensureWorkloadClusterServiceAccountIssuerInput{
																BootstrapProxy:       proxy,
																WorkloadProxy:        workloadProxy,
																Namespace:            namespace,
																ClusterName:          clusterName,
																ServiceAccountIssuer: saIssuerConfig,
																ControlPlaneIntervals: testE2EConfig.GetIntervals(
																	flavor,
																	"wait-control-plane",
																),
															},
														)
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

func enableMockOIDCInFlavorTemplate(
	testE2EConfig *clusterctl.E2EConfig,
	lowercaseProvider, flavor, tmpDir string,
) *oidc.Bundle {
	target := "cluster-template-" + flavor + ".yaml"
	for i := range testE2EConfig.Providers {
		p := &testE2EConfig.Providers[i]
		if p.Name != lowercaseProvider {
			continue
		}
		for j := range p.Versions {
			v := &p.Versions[j]
			for k := range v.Files {
				f := &v.Files[k]
				if f.TargetName != target {
					continue
				}
				raw, err := os.ReadFile(f.SourcePath)
				Expect(err).ToNot(HaveOccurred(), "failed to read template %q", f.SourcePath)

				serviceCIDR, err := oidc.ResolveClusterTemplateServiceCIDR(
					raw,
					func(name string) (string, bool) {
						if envVal, ok := os.LookupEnv(name); ok && envVal != "" {
							return envVal, true
						}
						if testE2EConfig != nil && testE2EConfig.HasVariable(name) {
							if val := testE2EConfig.GetVariableOrEmpty(name); val != "" {
								return val, true
							}
						}
						return "", false
					},
				)
				Expect(err).ToNot(HaveOccurred(), "resolving service CIDR for %q", f.SourcePath)

				staticIP, err := oidc.StaticServiceIPFromCIDR(serviceCIDR)
				Expect(err).ToNot(HaveOccurred(), "selecting static OIDC IP from %q", serviceCIDR)

				bundle, err := oidc.NewBundle(staticIP)
				Expect(err).ToNot(HaveOccurred(), "generating pre-boot mock OIDC bundle")

				patched, err := oidc.EnableAuthenticationInClusterTemplate(
					raw,
					bundle.AuthenticationConfig(),
				)
				Expect(err).ToNot(HaveOccurred(), "injecting authentication into %q", f.SourcePath)

				dst := filepath.Join(tmpDir, target)
				Expect(os.WriteFile(dst, patched, 0o600)).To(Succeed())
				f.SourcePath = dst
				Logf(
					"Pre-configured mock OIDC %s (service CIDR %s) in %s/%s",
					bundle.IssuerURL,
					serviceCIDR,
					lowercaseProvider,
					target,
				)
				return bundle
			}
		}
	}
	Fail(fmt.Sprintf("flavor %q not found for provider %q", flavor, lowercaseProvider))
	return nil
}

type ensureWorkloadClusterAuthenticationInput struct {
	BootstrapProxy        capiframework.ClusterProxy
	WorkloadProxy         capiframework.ClusterProxy
	WorkloadCluster       *clusterv1.Cluster
	Namespace             string
	ClusterName           string
	Bundle                *oidc.Bundle
	ControlPlaneIntervals []any
	DeploymentIntervals   []any
}

func ensureWorkloadClusterAuthentication(
	ctx context.Context,
	input ensureWorkloadClusterAuthenticationInput,
) {
	By("Validating structured authentication with pre-generated CA and static Service ClusterIP")
	Expect(input.Bundle).ToNot(BeNil())
	Expect(oidc.VerifyServiceIPInCluster(input.Bundle.ClusterIP, input.WorkloadCluster)).To(Succeed())

	bootstrapClient := input.BootstrapProxy.GetClient()
	Eventually(func() bool {
		c := &clusterv1.Cluster{}
		if err := bootstrapClient.Get(
			ctx,
			client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName},
			c,
		); err != nil {
			Logf("failed to get Cluster: %v", err)
			return false
		}
		if !oidc.ClusterControlPlaneSettled(c) {
			Logf(
				"Cluster control plane not settled (generation %d observed %d), conditions: %v",
				c.Generation,
				c.Status.ObservedGeneration,
				c.Status.Conditions,
			)
			return false
		}
		kcpList := &controlplanev1.KubeadmControlPlaneList{}
		if err := bootstrapClient.List(
			ctx,
			kcpList,
			client.InNamespace(input.Namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: input.ClusterName},
		); err != nil {
			Logf("failed to list KubeadmControlPlanes: %v", err)
			return false
		}
		for i := range kcpList.Items {
			kcp := &kcpList.Items[i]
			if oidc.AuthenticationConfigCurrent(kcp) {
				return true
			}
			Logf(
				"KCP %s generation %d observed %d auth file %t replicas %d ready %d upToDate %d",
				kcp.Name,
				kcp.Generation,
				kcp.Status.ObservedGeneration,
				oidc.KubeadmControlPlaneHasFile(kcp, authentication.AuthenticationConfigPath),
				ptr.Deref(kcp.Status.Replicas, 0),
				ptr.Deref(kcp.Status.ReadyReplicas, 0),
				ptr.Deref(kcp.Status.UpToDateReplicas, 0),
			)
		}
		return false
	}, input.ControlPlaneIntervals...).Should(
		BeTrue(),
		"KubeadmControlPlane did not settle with authentication config",
	)

	workloadClient := input.WorkloadProxy.GetClient()
	By(fmt.Sprintf("Deploying mock OIDC at %s", input.Bundle.IssuerURL))
	Expect(input.Bundle.Deploy(ctx, workloadClient, oidc.DefaultNamespace)).To(Succeed())

	Eventually(func() bool {
		dep := &appsv1.Deployment{}
		if err := workloadClient.Get(
			ctx,
			client.ObjectKey{Namespace: oidc.DefaultNamespace, Name: oidc.ServiceName},
			dep,
		); err != nil {
			Logf("failed to get mock-oidc deployment: %v", err)
			return false
		}
		if dep.Status.ObservedGeneration >= dep.Generation {
			for _, c := range dep.Status.Conditions {
				if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
					return true
				}
			}
		}
		Logf(
			"mock-oidc deployment not yet available: generation %d observed %d replicas %d/%d/%d conditions %v",
			dep.Generation,
			dep.Status.ObservedGeneration,
			dep.Status.Replicas,
			dep.Status.ReadyReplicas,
			dep.Status.AvailableReplicas,
			dep.Status.Conditions,
		)
		return false
	}, input.DeploymentIntervals...).Should(
		BeTrue(),
		"mock OIDC deployment not available",
	)

	By("Verifying admin kubeconfig still works with authentication enabled")
	pods := &corev1.PodList{}
	Expect(workloadClient.List(ctx, pods, client.InNamespace("kube-system"))).To(Succeed())

	By("Validating valid JWT maps username, groups, and extra claims")
	validToken, err := input.Bundle.MintJWT(map[string]any{
		"email":  "test@example.com",
		"groups": []string{"e2e-group"},
		"hd":     "example.com",
		"foo":    "bar",
		"aud":    "e2e-aud",
		"sub":    "123",
	})
	Expect(err).ToNot(HaveOccurred())
	wantUser := &oidc.ExpectedUserInfo{
		Username: "test@example.com",
		Groups:   []string{"e2e-group"},
		Extra: map[string][]string{
			"example.org/foo": {"bar"},
		},
	}
	restCfg := input.WorkloadProxy.GetRESTConfig()
	Eventually(func() error {
		return oidc.ValidateTokenAuth(ctx, restCfg, validToken, wantUser)
	}, 2*time.Minute, 5*time.Second).Should(Succeed())

	By("Validating invalid JWTs are rejected")
	invalidCases := []map[string]any{
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
		{
			"email": "system:evil@example.com",
			"hd":    "example.com",
			"aud":   "e2e-aud",
		},
	}
	for _, claims := range invalidCases {
		token, err := input.Bundle.MintJWT(claims)
		Expect(err).ToNot(HaveOccurred())
		Expect(oidc.ValidateTokenAuth(ctx, restCfg, token, nil)).To(Succeed())
	}
}

func enableServiceAccountIssuerInFlavorTemplate(
	testE2EConfig *clusterctl.E2EConfig,
	lowercaseProvider, flavor, tmpDir string,
) *v1alpha1.ServiceAccountIssuer {
	target := "cluster-template-" + flavor + ".yaml"
	for i := range testE2EConfig.Providers {
		p := &testE2EConfig.Providers[i]
		if p.Name != lowercaseProvider {
			continue
		}
		for j := range p.Versions {
			v := &p.Versions[j]
			for k := range v.Files {
				f := &v.Files[k]
				if f.TargetName != target {
					continue
				}
				raw, err := os.ReadFile(f.SourcePath)
				Expect(err).ToNot(HaveOccurred(), "failed to read template %q", f.SourcePath)

				sai := oidc.DefaultE2EServiceAccountIssuer()
				patched, err := oidc.EnableServiceAccountIssuerInClusterTemplate(raw, sai)
				Expect(err).ToNot(HaveOccurred(), "injecting serviceAccountIssuer into %q", f.SourcePath)

				dst := filepath.Join(tmpDir, target)
				Expect(os.WriteFile(dst, patched, 0o600)).To(Succeed())
				f.SourcePath = dst
				Logf(
					"Pre-configured serviceAccountIssuer %s in %s/%s",
					sai.Issuer,
					lowercaseProvider,
					target,
				)
				return &sai
			}
		}
	}
	Fail(fmt.Sprintf("flavor %q not found for provider %q", flavor, lowercaseProvider))
	return nil
}

type ensureWorkloadClusterServiceAccountIssuerInput struct {
	BootstrapProxy        capiframework.ClusterProxy
	WorkloadProxy         capiframework.ClusterProxy
	Namespace             string
	ClusterName           string
	ServiceAccountIssuer  *v1alpha1.ServiceAccountIssuer
	ControlPlaneIntervals []any
}

func ensureWorkloadClusterServiceAccountIssuer(
	ctx context.Context,
	input ensureWorkloadClusterServiceAccountIssuerInput,
) {
	By("Validating ServiceAccount issuer configuration without external services")
	Expect(input.ServiceAccountIssuer).ToNot(BeNil())

	bootstrapClient := input.BootstrapProxy.GetClient()
	Eventually(func() bool {
		c := &clusterv1.Cluster{}
		if err := bootstrapClient.Get(
			ctx,
			client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName},
			c,
		); err != nil {
			Logf("failed to get Cluster: %v", err)
			return false
		}
		if !oidc.ClusterControlPlaneSettled(c) {
			Logf(
				"Cluster control plane not settled (generation %d observed %d), conditions: %v",
				c.Generation,
				c.Status.ObservedGeneration,
				c.Status.Conditions,
			)
			return false
		}
		kcpList := &controlplanev1.KubeadmControlPlaneList{}
		if err := bootstrapClient.List(
			ctx,
			kcpList,
			client.InNamespace(input.Namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: input.ClusterName},
		); err != nil {
			Logf("failed to list KubeadmControlPlanes: %v", err)
			return false
		}
		for i := range kcpList.Items {
			kcp := &kcpList.Items[i]
			if oidc.ServiceAccountIssuerConfigCurrent(kcp, *input.ServiceAccountIssuer) {
				return true
			}
			Logf(
				"KCP %s generation %d observed %d replicas %d ready %d upToDate %d extraArgs %v",
				kcp.Name,
				kcp.Generation,
				kcp.Status.ObservedGeneration,
				ptr.Deref(kcp.Status.Replicas, 0),
				ptr.Deref(kcp.Status.ReadyReplicas, 0),
				ptr.Deref(kcp.Status.UpToDateReplicas, 0),
				kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs,
			)
		}
		return false
	}, input.ControlPlaneIntervals...).Should(
		BeTrue(),
		"KubeadmControlPlane did not settle with ServiceAccount issuer extraArgs",
	)

	By("Verifying admin kubeconfig works with ServiceAccount issuer configured")
	workloadClient := input.WorkloadProxy.GetClient()
	Expect(workloadClient.List(ctx, &corev1.PodList{}, client.InNamespace("kube-system"))).To(Succeed())

	By("Verifying OIDC discovery, JWKS token signatures, audience enforcement, and maxTokenExpiration")
	restCfg := input.WorkloadProxy.GetRESTConfig()
	Eventually(func() error {
		return oidc.VerifyWorkloadServiceAccountIssuer(ctx, restCfg, *input.ServiceAccountIssuer)
	}, 2*time.Minute, 5*time.Second).Should(Succeed())
}
