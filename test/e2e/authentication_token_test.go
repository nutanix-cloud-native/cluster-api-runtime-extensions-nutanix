//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/helpers/oidc"
)

var _ = Describe("Authentication with mock OIDC", Label("authentication"), func() {
	var (
		testE2EConfig                    *clusterctl.E2EConfig
		clusterLocalClusterctlConfigPath string
	)

	BeforeEach(func() {
		testE2EConfig = e2eConfig.DeepCopy()
		infraProviders := testE2EConfig.InfrastructureProviders()
		Expect(infraProviders).NotTo(BeEmpty(), "no infrastructure providers in e2e config")
		lowercaseProvider := infraProviders[0]
		for _, p := range infraProviders {
			if p == "docker" {
				lowercaseProvider = p
				break
			}
		}
		applyProviderKubernetesVersionOverride(testE2EConfig, lowercaseProvider)
		if lowercaseProvider == "nutanix" {
			reserveNutanixIPsForCluster(testE2EConfig)
		}

		clusterLocalTempDir, err := os.MkdirTemp("", "clusterctl-auth-")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			Expect(os.RemoveAll(clusterLocalTempDir)).To(Succeed())
		})
		clusterLocalClusterctlConfigPath = createClusterctlLocalRepository(
			testE2EConfig,
			clusterLocalTempDir,
		)
	})

	It("validates JWTs via in-cluster mock OIDC with no external service", func() {
		capie2e.QuickStartSpec(ctx, func() capie2e.QuickStartSpecInput {
			infraProvider := testE2EConfig.InfrastructureProviders()[0]
			for _, p := range testE2EConfig.InfrastructureProviders() {
				if p == "docker" {
					infraProvider = p
					break
				}
			}
			flavor := "topology-cilium-helm-addon"

			return capie2e.QuickStartSpecInput{
				E2EConfig:              testE2EConfig,
				ClusterctlConfigPath:   clusterLocalClusterctlConfigPath,
				BootstrapClusterProxy:  bootstrapClusterProxy,
				ArtifactFolder:         artifactFolder,
				SkipCleanup:            skipCleanup,
				Flavor:                 ptr.To(flavor),
				InfrastructureProvider: ptr.To(infraProvider),
				PostMachinesProvisioned: func(proxy capiframework.ClusterProxy, namespace, clusterName string) {
					By("Deploying mock OIDC into workload cluster")
					workloadProxy := proxy.GetWorkloadCluster(ctx, namespace, clusterName)
					workloadClient := workloadProxy.GetClient()
					mock, err := oidc.SetupMockOIDC(ctx, oidc.SetupInput{
						WorkloadClient: workloadClient,
						Namespace:      "oidc-mock",
					})
					Expect(err).ToNot(HaveOccurred(), "setting up mock OIDC")
					By(fmt.Sprintf("Mock OIDC issuer: %s", mock.IssuerURL))

					Eventually(func() bool {
						dep := &appsv1.Deployment{}
						if err := workloadClient.Get(ctx, client.ObjectKey{Namespace: "oidc-mock", Name: "mock-oidc"}, dep); err != nil {
							return false
						}
						for _, c := range dep.Status.Conditions {
							if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
								return true
							}
						}
						return false
					}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "mock OIDC deployment not available")

					By("Patching Cluster with authentication configuration")
					bootstrapClient := proxy.GetClient()
					cluster := &clusterv1beta2.Cluster{}
					Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, cluster)).To(Succeed())
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
									Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
									Groups:   &v1alpha1.PrefixedClaimOrExpression{Claim: "groups"},
									Extra: []v1alpha1.ExtraMapping{
										{Key: "example.org/foo", ValueExpression: "claims.foo"},
									},
								},
								ClaimValidationRules: []v1alpha1.ClaimValidationRule{
									{Claim: "hd", RequiredValue: "example.com"},
								},
								UserValidationRules: []v1alpha1.UserValidationRule{
									{Expression: "!user.username.startsWith('system:')", Message: "system: prefix reserved"},
								},
							},
						},
					}
					cluster.Spec.Topology.Variables = upsertAuthenticationVariable(cluster.Spec.Topology.Variables, auth)
					Expect(bootstrapClient.Update(ctx, cluster)).To(Succeed())

					By("Waiting for Cluster to be Ready after authentication patch")
					Eventually(func() bool {
						c := &clusterv1beta2.Cluster{}
						if err := bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, c); err != nil {
							return false
						}
						for _, cond := range c.Status.Conditions {
							if cond.Type == clusterv1beta2.ReadyCondition && cond.Status == metav1.ConditionTrue {
								return true
							}
						}
						return false
					}, 10*time.Minute, 10*time.Second).Should(BeTrue(), "Cluster not Ready after authentication patch")

					By("Verifying admin kubeconfig still works")
					workloadClient2 := workloadProxy.GetClient()
					pods := &corev1.PodList{}
					Expect(workloadClient2.List(ctx, pods, client.InNamespace("kube-system"))).To(Succeed())

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
					Expect(validateTokenAuth(ctx, workloadProxy, validToken, true)).To(Succeed())

					By("Validating invalid JWTs are rejected")
					invalidCases := []map[string]interface{}{
						{"email": "test@example.com", "groups": []string{"e2e-group"}, "hd": "wrong.com", "aud": "e2e-aud"},
						{"email": "test@example.com", "hd": "example.com", "aud": "wrong-aud"},
						{"email": "test@example.com", "hd": "example.com", "aud": "e2e-aud", "exp": time.Now().Add(-time.Hour).Unix()},
					}
					for _, claims := range invalidCases {
						token, err := mock.MintJWT(claims)
						Expect(err).ToNot(HaveOccurred())
						Expect(validateTokenAuth(ctx, workloadProxy, token, false)).To(Succeed())
					}

					By("Validating system: prefix is rejected via userValidationRule")
					sysClaims := map[string]interface{}{
						"email": "system:evil@example.com",
						"hd":    "example.com",
						"aud":   "e2e-aud",
					}
					sysToken, err := mock.MintJWT(sysClaims)
					Expect(err).ToNot(HaveOccurred())
					Expect(validateTokenAuth(ctx, workloadProxy, sysToken, false)).To(Succeed())
				},
			}
		})
	})
})

func upsertAuthenticationVariable(vars []clusterv1beta2.ClusterVariable, auth v1alpha1.Authentication) []clusterv1beta2.ClusterVariable {
	// Convert Authentication to unstructured for JSON merge.
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
	// Not found, create new.
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
	if strings.Contains(err.Error(), "Unauthorized") || strings.Contains(err.Error(), "Forbidden") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
		return nil
	}
	return fmt.Errorf("expected Unauthorized/Forbidden but got: %w", err)
}
