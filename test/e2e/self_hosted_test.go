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

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/framework"
)

var _ = Describe("Self-hosted", Serial, func() {
	for _, provider := range []string{"Docker", "Nutanix"} {
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
					Label("provider:"+provider),
					Label("cni:"+cniProvider),
					Label("addonStrategy:"+addonStrategy),
					func() {
						framework.SelfHostedSpec(ctx, func() framework.SelfHostedSpecInput {
							return framework.SelfHostedSpecInput{
								E2EConfig:              e2eConfig,
								ClusterctlConfigPath:   clusterctlConfigPath,
								BootstrapClusterProxy:  bootstrapClusterProxy,
								ArtifactFolder:         artifactFolder,
								SkipCleanup:            skipCleanup,
								Flavor:                 flavour,
								InfrastructureProvider: ptr.To(lowercaseProvider),
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
									Expect(workloadCluster.Spec.Topology).ToNot(BeNil())
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

									WaitForCoreDNSToBeReadyInWorkloadCluster(
										ctx,
										WaitForCoreDNSToBeReadyInWorkloadClusterInput{
											WorkloadCluster: workloadCluster,
											ClusterProxy:    proxy,
											DeploymentIntervals: e2eConfig.GetIntervals(
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
			}
		}
	}
})
