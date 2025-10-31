// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestWebhookBehaviour(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{Topology: &clusterv1.Topology{
			Class:   "dummy-class",
			Version: "dummy-version",
		}},
	}

	g.Expect(env.Client.Create(ctx, cluster)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, cluster)).To(Succeed())
	})

	// Validate the cluster has been assigned a UUID.
	g.Expect(cluster.Annotations).
		To(HaveKeyWithValue(v1alpha1.ClusterUUIDAnnotationKey, Not(BeEmpty())))
	assignedUUID := cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey]

	// Validate that changing the UUID is not allowed.
	cluster.Annotations[v1alpha1.ClusterUUIDAnnotationKey] = "new-uuid"
	g.Expect(env.Client.Update(ctx, cluster)).ToNot(Succeed())

	// Validate that removing the UUID annotation is not allowed and the annotation is retained.
	delete(cluster.Annotations, v1alpha1.ClusterUUIDAnnotationKey)
	g.Expect(cluster.Annotations).NotTo(HaveKey(v1alpha1.ClusterUUIDAnnotationKey))
	g.Expect(env.Client.Update(ctx, cluster)).To(Succeed())
	g.Expect(cluster.Annotations).
		To(HaveKeyWithValue(v1alpha1.ClusterUUIDAnnotationKey, assignedUUID))
}

func TestWebhookSkipsClusterWithNilTopology(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    metav1.NamespaceDefault,
		},
	}

	g.Expect(env.Client.Create(ctx, cluster)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, cluster)).To(Succeed())
	})

	// Validate the cluster has not been assigned a UUID.
	g.Expect(cluster.Annotations).
		NotTo(HaveKey(v1alpha1.ClusterUUIDAnnotationKey))
}

func TestUUIDIsAddedToAPreexistingClusterWhenTopologyIsAdded(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting-without-topology-or-uuid-annotation",
			Namespace: metav1.NamespaceDefault,
		},
	}

	g.Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
	g.Expect(cluster.Annotations).ToNot(HaveKey(v1alpha1.ClusterUUIDAnnotationKey))

	// Validate that the webhook does assign a UUID to the cluster if the Cluster has topology added.
	cluster.Spec.Topology = &clusterv1.Topology{
		Class:   "dummy-class",
		Version: "dummy-version",
	}
	g.Expect(env.Client.Update(ctx, cluster)).To(Succeed())
	g.Expect(cluster.Annotations).
		To(HaveKeyWithValue(v1alpha1.ClusterUUIDAnnotationKey, Not(BeEmpty())))
}

func TestUUIDIsAddedToAPreexistingClusterWhenTopologyIsUpdated(t *testing.T) {
	g := NewWithT(t)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting-with-topology-without-uuid-annotation",
			Namespace: metav1.NamespaceDefault,
		},
	}

	g.Expect(env.Client.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
	g.Expect(cluster.Annotations).ToNot(HaveKey(v1alpha1.ClusterUUIDAnnotationKey))
	g.Expect(cluster.Spec.Topology).ToNot(BeNil())

	// Validate that the webhook does assign a UUID to the cluster if the topology is updated. This could be when
	// a cluster changes to use CAREN for example, but we just change topology version here as an example of
	// a change that would trigger the webhook.
	cluster.Spec.Topology.Version = "dummy-version"
	g.Expect(env.Client.Update(ctx, cluster)).To(Succeed())
	g.Expect(cluster.Annotations).
		To(HaveKeyWithValue(v1alpha1.ClusterUUIDAnnotationKey, Not(BeEmpty())))
}
