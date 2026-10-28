// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func newStorageEnvtestCluster(pools ...nodePool) *clusterv1beta2.Cluster {
	cluster := newStorageTestCluster("nutanix", pools...)
	cluster.ObjectMeta = metav1.ObjectMeta{
		GenerateName: "test-storage-cluster-",
		Namespace:    metav1.NamespaceDefault,
		Labels:       cluster.Labels,
	}
	return cluster
}

func TestWebhookRejectsCreatingMultipleStorageNodePools(t *testing.T) {
	g := NewWithT(t)

	cluster := newStorageEnvtestCluster(
		nodePool{name: "md-0"},
		nodePool{name: "storage-a", storage: storageOnly},
		nodePool{name: "storage-b", storage: computeAndStorage},
	)

	err := env.Client.Create(ctx, cluster)
	g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid error, got %v", err)
	g.Expect(err.Error()).To(And(
		ContainSubstring(`spec.topology.workers.machineDeployments[1]: Forbidden: node pool "storage-a"`),
		ContainSubstring(`spec.topology.workers.machineDeployments[2]: Forbidden: node pool "storage-b"`),
	))
}

func TestWebhookRejectsAddingSecondStorageNodePool(t *testing.T) {
	g := NewWithT(t)

	cluster := newStorageEnvtestCluster(
		nodePool{name: "storage-a", storage: storageOnly},
		nodePool{name: "md-0"},
	)
	g.Expect(env.Client.Create(ctx, cluster)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, cluster)).To(Succeed())
	})

	cluster.Spec.Topology.Workers.MachineDeployments[1].Metadata.Labels = map[string]string{
		storageNodePoolLabelKey: computeAndStorage,
	}
	err := env.Client.Update(ctx, cluster)
	g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid error, got %v", err)
	g.Expect(err.Error()).To(ContainSubstring(
		`spec.topology.workers.machineDeployments[1]: Forbidden: node pool "md-0"`,
	))
}

func TestWebhookRejectsCreatingStorageNodePoolWithNutanixCSI(t *testing.T) {
	g := NewWithT(t)

	cluster := withCSIProviders(
		newStorageEnvtestCluster(
			nodePool{name: "md-0"},
			nodePool{name: "storage", storage: computeAndStorage},
		),
		v1alpha1.CSIProviderNutanix,
	)

	err := env.Client.Create(ctx, cluster)
	g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid error, got %v", err)
	g.Expect(err.Error()).To(ContainSubstring(
		`spec.topology.workers.machineDeployments[1]: Forbidden: node pool "storage" is a storage node pool`,
	))
}

func TestWebhookRejectsAddingNutanixCSIToClusterWithStorageNodePool(t *testing.T) {
	g := NewWithT(t)

	cluster := newStorageEnvtestCluster(nodePool{name: "storage", storage: storageOnly})
	g.Expect(env.Client.Create(ctx, cluster)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, cluster)).To(Succeed())
	})

	err := env.Client.Update(ctx, withCSIProviders(cluster, v1alpha1.CSIProviderNutanix))
	g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid error, got %v", err)
	g.Expect(err.Error()).To(ContainSubstring(
		`spec.topology.workers.machineDeployments[0]: Forbidden: node pool "storage" is a storage node pool`,
	))
}
