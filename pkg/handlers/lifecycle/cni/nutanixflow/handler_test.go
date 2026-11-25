// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanixflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	remotefake "sigs.k8s.io/cluster-api/controllers/remote/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestFlowNamespaces(t *testing.T) {
	t.Parallel()

	// Flow CNI 1.1.0 deploys into three namespaces. The handler always ensures
	// they exist (including when imagePullCredentials are omitted for a
	// registry mirror), and copies credentials into each when provided.
	assert.ElementsMatch(t, []string{
		"flow-cni-system",
		"flow-cns-system",
		"ovn-kubernetes",
	}, flowNamespaces)
}

func TestPrepareRemoteClusterForFlow_WithoutImagePullCredentials(t *testing.T) {
	t.Parallel()

	scheme := newFlowTestScheme()
	cluster := newFlowTestCluster()
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()

	secretName, err := prepareRemoteClusterForFlow(
		context.Background(),
		c,
		remotefake.NewClusterClient,
		cluster,
		v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderFlow,
			Strategy: v1alpha1.AddonStrategyHelmAddon,
		},
	)
	require.NoError(t, err)
	assert.Empty(t, secretName)

	for _, ns := range flowNamespaces {
		got := &corev1.Namespace{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: ns}, got))

		secret := &corev1.Secret{}
		err := c.Get(context.Background(), client.ObjectKey{
			Name:      defaultImagePullSecretName,
			Namespace: ns,
		}, secret)
		assert.True(t, apierrors.IsNotFound(err), "expected no pull secret in %s", ns)
	}
}

func TestPrepareRemoteClusterForFlow_WithImagePullCredentials(t *testing.T) {
	t.Parallel()

	scheme := newFlowTestScheme()
	cluster := newFlowTestCluster()
	sourceSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-flow-image-pull-secret",
			Namespace: cluster.Namespace,
		},
		Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{"username":"u","password":"p"}}}`),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, sourceSecret).Build()

	secretName, err := prepareRemoteClusterForFlow(
		context.Background(),
		c,
		remotefake.NewClusterClient,
		cluster,
		v1alpha1.CNI{
			Provider: v1alpha1.CNIProviderFlow,
			Strategy: v1alpha1.AddonStrategyHelmAddon,
			ImagePullCredentials: &v1alpha1.ImagePullCredentials{
				SecretRef: v1alpha1.LocalObjectReference{Name: sourceSecret.Name},
			},
		},
	)
	require.NoError(t, err)
	assert.Equal(t, defaultImagePullSecretName, secretName)

	for _, ns := range flowNamespaces {
		got := &corev1.Namespace{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: ns}, got))

		secret := &corev1.Secret{}
		require.NoError(t, c.Get(context.Background(), client.ObjectKey{
			Name:      defaultImagePullSecretName,
			Namespace: ns,
		}, secret))
		assert.Equal(t, corev1.SecretTypeDockerConfigJson, secret.Type)
		assert.Equal(t, sourceSecret.Data[corev1.DockerConfigJsonKey], secret.Data[corev1.DockerConfigJsonKey])
	}
}

func newFlowTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	return scheme
}

func newFlowTestCluster() *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-flow",
			Namespace: metav1.NamespaceDefault,
			UID:       "test-flow-uid",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: "infrastructure.cluster.x-k8s.io",
				Kind:     "NutanixCluster",
				Name:     "dummy",
			},
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "dummy-class"},
				Version:  "v1.30.0",
			},
		},
	}
}
