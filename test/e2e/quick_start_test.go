//go:build e2e

// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"

	caaphv1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
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

var _ = Describe("Docker examples [Docker]", Serial, func() {
	for _, specName := range []string{
		"topology-cilium-helm-addon [HelmAddon][Cilium]",
		"topology-cilium-crs [ClusterResourceSet][Cilium]",
		"topology-calico-helm-addon [HelmAddon][Calico]",
		"topology-calico-crs [ClusterResourceSet][Calico]",
	} {
		flavour, _, found := strings.Cut(specName, " ")
		Expect(found).To(BeTrue(), "template has invalid format")
		Context(specName, func() {
			e2e.QuickStartSpec(ctx, func() e2e.QuickStartSpecInput {
				return e2e.QuickStartSpecInput{
					E2EConfig:              e2eConfig,
					ClusterctlConfigPath:   clusterctlConfigPath,
					BootstrapClusterProxy:  bootstrapClusterProxy,
					ArtifactFolder:         artifactFolder,
					SkipCleanup:            skipCleanup,
					Flavor:                 ptr.To(flavour),
					InfrastructureProvider: ptr.To("docker"),
					PostMachinesProvisioned: func(proxy framework.ClusterProxy, namespace, clusterName string) {
						framework.AssertOwnerReferences(namespace, proxy.GetKubeconfigPath(),
							framework.CoreOwnerReferenceAssertion,
							framework.ExpOwnerReferenceAssertions,
							framework.DockerInfraOwnerReferenceAssertions,
							framework.KubeadmBootstrapOwnerReferenceAssertions,
							framework.KubeadmControlPlaneOwnerReferenceAssertions,
							framework.KubernetesReferenceAssertions,
							map[string]func(reference []metav1.OwnerReference) error{
								clusterResourceSetKind: func(owners []metav1.OwnerReference) error {
									// The ClusterResourcesSets that we create are cluster specific and so should be owned by the cluster.
									return framework.HasExactOwners(owners, clusterOwner)
								},

								helmChartProxyKind: func(owners []metav1.OwnerReference) error {
									// The HelmChartProxies that we create are cluster specific and so should be owned by the cluster.
									return framework.HasExactOwners(owners, clusterOwner)
								},

								helmReleaseProxyKind: func(owners []metav1.OwnerReference) error {
									// HelmReleaseProxies should be owned by the relevant HelmChartProxy.
									return framework.HasExactOwners(owners, helmChartProxyOwner)
								},
							},
						)

						By("Waiting until nodes are ready")
						workloadProxy := proxy.GetWorkloadCluster(ctx, namespace, clusterName)
						workloadClient := workloadProxy.GetClient()
						framework.WaitForNodesReady(ctx, framework.WaitForNodesReadyInput{
							Lister:            workloadClient,
							KubernetesVersion: e2eConfig.GetVariable(KubernetesVersion),
							Count:             2,
							WaitForNodesReady: e2eConfig.GetIntervals(specName, "wait-nodes-ready"),
						})
					},
				}
			})
		})
	}
})
