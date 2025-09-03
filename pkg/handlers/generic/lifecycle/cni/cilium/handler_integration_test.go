// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/lifecycle/addons"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("Test runApply", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	It("should clean kube-proxy when neccesery", func(ctx SpecContext) {
		c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		cluster, remoteClient := setupTestCluster(ctx, c)
		strategy := addons.NewTestStrategy(nil)

		By("Should not delete kube-proxy when skip kube-proxy is not set")
		err = runApply(ctx, c, cluster, strategy, cluster.Namespace, logr.Discard())
		Expect(err).To(BeNil())

		// Verify that the kube-proxy DaemonSet and ConfigMap are not deleted
		daemonSet := &appsv1.DaemonSet{}
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, daemonSet)
		Expect(err).To(BeNil())
		Expect(daemonSet).ToNot(BeNil())
		configMap := &corev1.ConfigMap{}
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, configMap)
		Expect(err).To(BeNil())
		Expect(configMap).ToNot(BeNil())

		By("Should not delete when the addon is not applied")
		err = runApply(
			ctx,
			c,
			cluster,
			addons.NewTestStrategy(fmt.Errorf("test error")),
			cluster.Namespace,
			logr.Discard(),
		)
		Expect(err).ToNot(BeNil())

		// Verify that the kube-proxy DaemonSet and ConfigMap are not deleted when the addon upgrade errors
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, daemonSet)
		Expect(err).To(BeNil())
		Expect(daemonSet).ToNot(BeNil())
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, configMap)
		Expect(err).To(BeNil())
		Expect(configMap).ToNot(BeNil())

		By("Should delete kube-proxy when skip kube-proxy is set")
		cluster.Spec.Topology.ControlPlane.Metadata.Annotations = map[string]string{
			controlplanev1.SkipKubeProxyAnnotation: "",
		}

		err = runApply(ctx, c, cluster, strategy, cluster.Namespace, logr.Discard())
		Expect(err).To(BeNil())

		// Verify that the kube-proxy DaemonSet and ConfigMap are deleted.
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, daemonSet)
		Expect(err).ToNot(BeNil())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, configMap)
		Expect(err).ToNot(BeNil())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		By("Should not fail when kube-proxy is not already installed")
		err = runApply(ctx, c, cluster, strategy, cluster.Namespace, logr.Discard())
		Expect(err).To(BeNil())
	})
})

func setupTestCluster(
	ctx SpecContext,
	c ctrlclient.Client,
) (*clusterv1.Cluster, ctrlclient.Client) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    corev1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Class:   "dummy-class",
				Version: "dummy-version",
			},
		},
	}
	Expect(c.Create(ctx, cluster)).To(Succeed())

	Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())
	remoteClient, err := remote.NewClusterClient(ctx, "", c, ctrlclient.ObjectKeyFromObject(cluster))
	Expect(err).To(BeNil())

	// Create kube-proxy DaemonSet and ConfigMap
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeProxyName,
			Namespace: kubeProxyNamespace,
			Labels: map[string]string{
				"app": kubeProxyName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": kubeProxyName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      kubeProxyName,
					Namespace: kubeProxyNamespace,
					Labels: map[string]string{
						"app": kubeProxyName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  kubeProxyName,
							Image: kubeProxyName,
						},
					},
				},
			},
		},
	}
	Expect(remoteClient.Create(ctx, daemonSet)).To(Succeed())

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeProxyName,
			Namespace: kubeProxyNamespace,
		},
	}
	Expect(remoteClient.Create(ctx, configMap)).To(Succeed())

	ciliumDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultCiliumReleaseName,
			Namespace: defaultCiliumNamespace,
			Labels: map[string]string{
				"app": defaultCiliumReleaseName,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": defaultCiliumReleaseName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultCiliumReleaseName,
					Namespace: defaultCiliumNamespace,
					Labels: map[string]string{
						"app": defaultCiliumReleaseName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  defaultCiliumReleaseName,
							Image: defaultCiliumReleaseName,
						},
					},
				},
			},
		},
	}
	Expect(remoteClient.Create(ctx, ciliumDaemonSet)).To(Succeed())

	return cluster, remoteClient
}
