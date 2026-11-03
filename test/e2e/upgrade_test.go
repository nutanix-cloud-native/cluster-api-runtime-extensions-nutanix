//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	capibootstrap "sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Upgrading CAREN must not change anything on existing Clusters: they keep the handler versions named in the
// ClusterClass they were created from.
var _ = Describe("Upgrade", Label("provider:Docker"), Serial, func() {
	var before topologyState

	capie2e.ClusterctlUpgradeSpec(ctx, func() capie2e.ClusterctlUpgradeSpecInput {
		return capie2e.ClusterctlUpgradeSpecInput{
			E2EConfig:                   e2eConfig,
			ClusterctlConfigPath:        clusterctlConfigPath,
			BootstrapClusterProxy:       bootstrapClusterProxy,
			ArtifactFolder:              artifactFolder,
			SkipCleanup:                 skipCleanup,
			UseKindForManagementCluster: true,
			InitWithBinary: fmt.Sprintf(
				"https://github.com/kubernetes-sigs/cluster-api/releases/download/%s/clusterctl-{OS}-{ARCH}",
				e2eConfig.MustGetVariable("CAPI_VERSION"),
			),
			// The kind node image must exist, so use the version CAPI's kind mapper pins.
			InitWithKubernetesVersion: strings.SplitN(capibootstrap.DefaultNodeImageVersion, "@", 2)[0],
			InitWithInfrastructureProviders: e2eConfig.GetProviderLatestVersionsByContract(
				"*",
				"docker",
			),
			InitWithRuntimeExtensionProviders: []string{
				"caren:" + e2eConfig.MustGetVariable("CAREN_UPGRADE_FROM_VERSION"),
			},
			Upgrades: []capie2e.ClusterctlUpgradeSpecInputUpgrade{{
				RuntimeExtensionProviders: e2eConfig.GetProviderLatestVersionsByContract("*", "caren"),
			}},
			InfrastructureProvider: ptr.To("docker"),
			WorkloadFlavor:         "upgrade-from-release",
			PreUpgrade: func(proxy capiframework.ClusterProxy) {
				before = getTopologyState(proxy)
			},
			// Runs before the spec's own check that no Machines were replaced.
			PostUpgrade: func(proxy capiframework.ClusterProxy, _, _ string) {
				// A changed desired state is applied before lifecycle hooks run, and the rollout it starts can make
				// those hooks fail, so compare before requiring a successful reconcile. Compare again afterwards in
				// case the first reconcile failed before applying anything.
				reconcileTopology(proxy)
				Expect(cmp.Diff(before, getTopologyState(proxy))).To(BeEmpty())
				waitForTopologyReconciled(proxy)
				Expect(cmp.Diff(before, getTopologyState(proxy))).To(BeEmpty())
			},
		}
	})
})

type topologyState struct {
	Variables          []clusterv1.ClusterVariable
	ControlPlanes      map[string]controlplanev1.KubeadmControlPlaneSpec
	MachineDeployments map[string]clusterv1.MachineDeploymentSpec
}

func getWorkloadCluster(proxy capiframework.ClusterProxy) *clusterv1.Cluster {
	clusters := &clusterv1.ClusterList{}
	Expect(proxy.GetClient().List(ctx, clusters)).To(Succeed())
	Expect(clusters.Items).To(HaveLen(1))
	return &clusters.Items[0]
}

func getTopologyState(proxy capiframework.ClusterProxy) topologyState {
	cluster := getWorkloadCluster(proxy)
	opts := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: cluster.Name},
	}

	state := topologyState{
		Variables:          cluster.Spec.Topology.Variables,
		ControlPlanes:      map[string]controlplanev1.KubeadmControlPlaneSpec{},
		MachineDeployments: map[string]clusterv1.MachineDeploymentSpec{},
	}

	kcps := &controlplanev1.KubeadmControlPlaneList{}
	Expect(proxy.GetClient().List(ctx, kcps, opts...)).To(Succeed())
	Expect(kcps.Items).ToNot(BeEmpty())
	for i := range kcps.Items {
		state.ControlPlanes[kcps.Items[i].Name] = kcps.Items[i].Spec
	}

	mds := &clusterv1.MachineDeploymentList{}
	Expect(proxy.GetClient().List(ctx, mds, opts...)).To(Succeed())
	Expect(mds.Items).ToNot(BeEmpty())
	for i := range mds.Items {
		state.MachineDeployments[mds.Items[i].Name] = mds.Items[i].Spec
	}

	return state
}

// reconcileTopology waits until the topology controller has reconciled the Cluster with the running extensions.
// Toggling spec.paused bumps the generation without changing the desired state, and the TopologyReconciled condition
// records the generation it last reconciled, whether or not that reconcile succeeded.
func reconcileTopology(proxy capiframework.ClusterProxy) {
	cluster := getWorkloadCluster(proxy)
	for _, paused := range []bool{true, false} {
		patch := client.MergeFrom(cluster.DeepCopy())
		cluster.Spec.Paused = ptr.To(paused)
		Expect(proxy.GetClient().Patch(ctx, cluster, patch)).To(Succeed())
		generation := cluster.Generation

		Eventually(func(g Gomega) {
			g.Expect(proxy.GetClient().Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
			condition := conditions.Get(cluster, clusterv1.ClusterTopologyReconciledCondition)
			g.Expect(condition).ToNot(BeNil())
			g.Expect(condition.ObservedGeneration).To(Equal(generation))
		}, e2eConfig.GetIntervals("upgrade", "wait-cluster")...).Should(Succeed())
	}
}

func waitForTopologyReconciled(proxy capiframework.ClusterProxy) {
	cluster := getWorkloadCluster(proxy)
	Eventually(func(g Gomega) {
		g.Expect(proxy.GetClient().Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		condition := conditions.Get(cluster, clusterv1.ClusterTopologyReconciledCondition)
		g.Expect(condition).ToNot(BeNil())
		g.Expect(condition.ObservedGeneration).To(Equal(cluster.Generation))
		g.Expect(condition.Status).To(Equal(metav1.ConditionTrue), condition.Message)
	}, e2eConfig.GetIntervals("upgrade", "wait-cluster")...).Should(Succeed())
}
