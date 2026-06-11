//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"os"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
)

// Automatic kubelet reservations are validated with a dedicated flavor generated at runtime
// from the published quick-start example: the worker nodes come up already configured, so the
// reservation is applied on first boot with no MachineDeployment rollout. The published
// examples are never modified. One combination (Cilium + HelmAddon) per provider is enough to
// exercise the boot-time mechanism end to end.
var _ = Describe("Automatic kubelet reservations", Label("kubelet-reservations"), func() {
	for provider := range providerConfigurations {
		var providerSpecificDecorators []interface{}
		if provider == "Docker" {
			providerSpecificDecorators = append(providerSpecificDecorators, Serial)
		}

		Context(provider, Label("provider:"+provider), providerSpecificDecorators, func() {
			lowercaseProvider := strings.ToLower(provider)
			baseFlavor := "topology-cilium-helm-addon"

			var (
				testE2EConfig                    *clusterctl.E2EConfig
				clusterLocalClusterctlConfigPath string
				flavor                           string
			)

			BeforeEach(func() {
				testE2EConfig = e2eConfig.DeepCopy()

				applyProviderKubernetesVersionOverride(testE2EConfig, lowercaseProvider)

				if provider == "Nutanix" {
					reserveNutanixIPsForCluster(testE2EConfig)
				}

				clusterLocalTempDir, err := os.MkdirTemp("", "clusterctl-kubelet-reservations-")
				Expect(err).ToNot(HaveOccurred())
				DeferCleanup(func() {
					Expect(os.RemoveAll(clusterLocalTempDir)).To(Succeed())
				})

				flavor = registerAutomaticReservationsFlavor(
					testE2EConfig,
					lowercaseProvider,
					baseFlavor,
					clusterLocalTempDir,
				)

				// Opt-in workaround for running workload clusters under rootless Docker.
				maybeEnableKubeletInUserNamespace(
					testE2EConfig,
					lowercaseProvider,
					clusterLocalTempDir,
				)

				clusterLocalClusterctlConfigPath = createClusterctlLocalRepository(
					testE2EConfig,
					clusterLocalTempDir,
				)
			})

			capie2e.QuickStartSpec(ctx, func() capie2e.QuickStartSpecInput {
				if !slices.Contains(e2eConfig.InfrastructureProviders(), lowercaseProvider) {
					Fail(fmt.Sprintf(
						"provider %s is not enabled - check environment setup for provider specific requirements",
						lowercaseProvider,
					))
				}

				clusterNamePrefix := "kubelet-res-"
				maxClusterNameLength := 63
				if lowercaseProvider == "docker" {
					maxClusterNameLength = 64 - 23
				}

				return capie2e.QuickStartSpecInput{
					ClusterName: ptr.To(clusterNamePrefix +
						util.RandomString(maxClusterNameLength-len(clusterNamePrefix))),
					E2EConfig:              testE2EConfig,
					ClusterctlConfigPath:   clusterLocalClusterctlConfigPath,
					BootstrapClusterProxy:  bootstrapClusterProxy,
					ArtifactFolder:         artifactFolder,
					SkipCleanup:            skipCleanup,
					Flavor:                 ptr.To(flavor),
					InfrastructureProvider: ptr.To(lowercaseProvider),
					PostMachinesProvisioned: func(proxy capiframework.ClusterProxy, namespace, clusterName string) {
						By("Verifying worker nodes report reserved CPU and memory")
						workloadProxy := proxy.GetWorkloadCluster(ctx, namespace, clusterName)
						assertWorkerNodesHaveReservedResources(
							ctx,
							workloadProxy.GetClient(),
							testE2EConfig.GetIntervals(flavor, "wait-nodes-ready"),
						)
					},
				}
			})
		})
	}
})
