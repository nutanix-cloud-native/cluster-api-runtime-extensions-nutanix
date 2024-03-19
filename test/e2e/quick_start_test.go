//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

var _ = Describe("Quick start", Serial, func() {
	for _, provider := range []string{"Docker", "AWS"} {
		lowercaseProvider := strings.ToLower(provider)
		for _, cniProvider := range []string{"Cilium", "Calico"} {
			for _, addonStrategy := range []string{"HelmAddon", "ClusterResourceSet"} {
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
					Label(
						"provider:"+provider,
					),
					Label("cni:"+cniProvider),
					Label("addonStrategy:"+addonStrategy),
					func() {
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
								E2EConfig:              e2eConfig,
								ClusterctlConfigPath:   clusterctlConfigPath,
								BootstrapClusterProxy:  bootstrapClusterProxy,
								ArtifactFolder:         artifactFolder,
								SkipCleanup:            skipCleanup,
								Flavor:                 ptr.To(flavour),
								InfrastructureProvider: ptr.To(lowercaseProvider),
								PostMachinesProvisioned: func(proxy framework.ClusterProxy, namespace, clusterName string) {
									framework.AssertOwnerReferences(
										namespace,
										proxy.GetKubeconfigPath(),
										framework.CoreOwnerReferenceAssertion,
										framework.ExpOwnerReferenceAssertions,
										framework.DockerInfraOwnerReferenceAssertions,
										framework.KubeadmBootstrapOwnerReferenceAssertions,
										framework.KubeadmControlPlaneOwnerReferenceAssertions,
										framework.KubernetesReferenceAssertions,
										AWSInfraOwnerReferenceAssertions,
										AddonReferenceAssertions,
									)

									By("Waiting until nodes are ready")
									workloadProxy := proxy.GetWorkloadCluster(
										ctx,
										namespace,
										clusterName,
									)
									workloadClient := workloadProxy.GetClient()
									framework.WaitForNodesReady(
										ctx,
										framework.WaitForNodesReadyInput{
											Lister: workloadClient,
											KubernetesVersion: e2eConfig.GetVariable(
												capie2e.KubernetesVersion,
											),
											Count: 2,
											WaitForNodesReady: e2eConfig.GetIntervals(
												flavour,
												"wait-nodes-ready",
											),
										},
									)

									By(
										"Waiting for all requested addons to be ready in workload cluster",
									)
									workloadCluster := framework.GetClusterByName(
										ctx,
										framework.GetClusterByNameInput{
											Namespace: namespace,
											Name:      clusterName,
											Getter:    proxy.GetClient(),
										},
									)
									Expect(workloadCluster.Spec.Topology).ToNot(BeNil())
									clusterVars := variables.ClusterVariablesToVariablesMap(
										workloadCluster.Spec.Topology.Variables,
									)
									addonsConfig, found, err := variables.Get[v1alpha1.Addons](
										clusterVars,
										clusterconfig.MetaVariableName,
										"addons",
									)
									Expect(err).ToNot(HaveOccurred())
									Expect(found).To(BeTrue())
									WaitForAddonsToBeReadyInWorkloadCluster(
										ctx,
										WaitForAddonsToBeReadyInWorkloadClusterInput{
											AddonsConfig:    addonsConfig,
											ClusterProxy:    proxy,
											WorkloadCluster: workloadCluster,
											DeploymentIntervals: e2eConfig.GetIntervals(
												flavour,
												"wait-deployment",
											),
											DaemonSetIntervals: e2eConfig.GetIntervals(
												flavour,
												"wait-daemonset",
											),
											HelmReleaseIntervals: e2eConfig.GetIntervals(
												flavour,
												"wait-helmrelease",
											),
											ClusterResourceSetIntervals: e2eConfig.GetIntervals(
												flavour,
												"wait-clusterresourceset",
											),
										},
									)
								},
							}
						})
					},
				)
			}
		}
	}
})
