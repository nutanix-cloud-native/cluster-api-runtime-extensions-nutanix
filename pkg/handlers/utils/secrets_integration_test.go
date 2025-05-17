// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Test EnsureSecretOnRemoteCluster", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	It("Secret should be created in the default namespace on the remote cluster", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())
		Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

		remoteClient, err := remote.NewClusterClient(ctx, "", c, ctrlclient.ObjectKeyFromObject(cluster))
		Expect(err).To(BeNil())

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: corev1.NamespaceDefault,
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}

		Expect(EnsureSecretOnRemoteCluster(ctx, c, secret, cluster)).To(Succeed())

		// Verify that the secret was created on the remote cluster.
		Expect(remoteClient.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), secret)).To(Succeed())
		Expect(secret.Name).To(Equal("test-secret"))
		Expect(secret.Namespace).To(Equal(corev1.NamespaceDefault))
		Expect(secret.Data).To(Equal(secret.Data))
	})

	It("Secret should be created in a new namespace on the remote cluster", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())
		Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

		remoteClient, err := remote.NewClusterClient(ctx, "", c, ctrlclient.ObjectKeyFromObject(cluster))
		Expect(err).To(BeNil())

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}

		Expect(EnsureSecretOnRemoteCluster(ctx, c, secret, cluster)).To(Succeed())

		// Verify that the secret was created on the remote cluster.
		Expect(remoteClient.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), secret)).To(Succeed())
		Expect(secret.Name).To(Equal("test-secret"))
		Expect(secret.Namespace).To(Equal("test-namespace"))
		Expect(secret.Data).To(Equal(secret.Data))
	})

	It("Should error if can't get remote cluster's kubeconfig", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
		}

		Expect(EnsureSecretOnRemoteCluster(ctx, c, secret, cluster)).To(
			MatchError(
				fmt.Sprintf("error creating client for remote cluster: "+
					"failed to retrieve kubeconfig secret for Cluster default/%s: secrets \"%s-kubeconfig\" not found",
					cluster.Name, cluster.Name),
			),
		)
	})
})

var _ = Describe("Test EnsureSecretForLocalCluster", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	It("Secret should be created in the cluster", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: corev1.NamespaceDefault,
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}

		Expect(EnsureSecretForLocalCluster(ctx, c, secret, cluster)).To(Succeed())

		// Verify that the secret was created on the local cluster.
		Expect(c.Get(ctx, ctrlclient.ObjectKeyFromObject(secret), secret)).To(Succeed())
		Expect(secret.OwnerReferences).To(
			ContainElement(
				metav1.OwnerReference{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       clusterv1.ClusterKind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			),
		)
	})
	It("Secret error if namespaces don't match", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-cluster-",
				Namespace:    corev1.NamespaceDefault,
			},
		}
		Expect(c.Create(ctx, cluster)).To(Succeed())

		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}

		Expect(EnsureSecretForLocalCluster(ctx, c, secret, cluster)).To(
			MatchError("secret namespace \"test-namespace\" does not match cluster namespace \"default\""),
		)
	})
})
