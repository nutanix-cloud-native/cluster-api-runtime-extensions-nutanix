//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"

	caaphv1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

var (
	clusterKind            = "Cluster"
	clusterResourceSetKind = "ClusterResourceSet"
	helmChartProxyKind     = "HelmChartProxy"
	helmReleaseProxyKind   = "HelmReleaseProxy"

	clusterOwner = metav1.OwnerReference{
		Kind:       clusterKind,
		APIVersion: clusterv1.GroupVersion.String(),
	}
	helmChartProxyOwner = metav1.OwnerReference{
		Kind:       helmChartProxyKind,
		APIVersion: caaphv1.GroupVersion.String(),
		Controller: ptr.To(true),
	}
)

var _ = Describe("Quick start", Serial, func() {
	for _, provider := range []string{"Docker"} {
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
						e2e.QuickStartSpec(ctx, func() e2e.QuickStartSpecInput {
							return e2e.QuickStartSpecInput{
								E2EConfig:              e2eConfig,
								ClusterctlConfigPath:   clusterctlConfigPath,
								BootstrapClusterProxy:  bootstrapClusterProxy,
								ArtifactFolder:         artifactFolder,
								SkipCleanup:            skipCleanup,
								Flavor:                 ptr.To(flavour),
								InfrastructureProvider: ptr.To(strings.ToLower(provider)),
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
										map[string]func(reference []metav1.OwnerReference) error{
											clusterResourceSetKind: func(owners []metav1.OwnerReference) error {
												// The ClusterResourcesSets that we create are cluster specific and so should be owned by the cluster.
												return framework.HasExactOwners(
													owners,
													clusterOwner,
												)
											},

											helmChartProxyKind: func(owners []metav1.OwnerReference) error {
												// The HelmChartProxies that we create are cluster specific and so should be owned by the cluster.
												return framework.HasExactOwners(
													owners,
													clusterOwner,
												)
											},

											helmReleaseProxyKind: func(owners []metav1.OwnerReference) error {
												// HelmReleaseProxies should be owned by the relevant HelmChartProxy.
												return framework.HasExactOwners(
													owners,
													helmChartProxyOwner,
												)
											},
										},
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
												KubernetesVersion,
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
