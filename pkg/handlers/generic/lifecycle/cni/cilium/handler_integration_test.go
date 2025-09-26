// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"fmt"
	"time"

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
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
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

		By("Should not delete kube-proxy when it is not disabled")
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

		By("Should not delete kube-proxy when the addon is not applied")
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

		By("Should delete kube-proxy when kube-proxy is disabled")
		err = disableKubeProxy(cluster)
		Expect(err).To(BeNil())

		// Speed up the test.
		waitTimeout = 1 * time.Second
		err = runApply(ctx, c, cluster, strategy, cluster.Namespace, logr.Discard())
		Expect(err).ToNot(BeNil())

		// Verify that the kube-proxy DaemonSet and ConfigMap are not deleted when Cilium DaemonSet is not updated
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, daemonSet)
		Expect(err).To(BeNil())
		Expect(daemonSet).ToNot(BeNil())
		err = remoteClient.Get(ctx, ctrlclient.ObjectKey{Name: kubeProxyName, Namespace: kubeProxyNamespace}, configMap)
		Expect(err).To(BeNil())
		Expect(configMap).ToNot(BeNil())

		By("Should delete kube-proxy when skip kube-proxy is set")
		// Update the status of the Cilium DaemonSet to simulate a roll out.
		ciliumDaemonSet := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultCiliumReleaseName,
				Namespace: defaultCiliumNamespace,
			},
		}
		err = remoteClient.Get(
			ctx,
			ctrlclient.ObjectKey{Name: defaultCiliumReleaseName, Namespace: defaultCiliumNamespace},
			ciliumDaemonSet,
		)
		Expect(err).To(BeNil())
		ciliumDaemonSet.Status = appsv1.DaemonSetStatus{
			ObservedGeneration:     2,
			NumberAvailable:        2,
			DesiredNumberScheduled: 2,
			UpdatedNumberScheduled: 2,
			NumberUnavailable:      0,
		}
		Expect(remoteClient.Status().Update(ctx, ciliumDaemonSet)).To(Succeed())

		err = runApply(ctx, c, cluster, strategy, cluster.Namespace, logr.Discard())
		Expect(err).To(BeNil())

		// Verify that the Cilium DaemonSet was updated.
		ciliumDaemonSet = &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      defaultCiliumReleaseName,
				Namespace: defaultCiliumNamespace,
			},
		}
		err = remoteClient.Get(
			ctx,
			ctrlclient.ObjectKeyFromObject(ciliumDaemonSet),
			ciliumDaemonSet,
		)
		Expect(err).To(BeNil())
		Expect(ciliumDaemonSet).ToNot(BeNil())
		Expect(ciliumDaemonSet.Spec.Template.Annotations).To(HaveKey(restartedAtAnnotation))

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

	// Cilium DaemonSet, Pods and ConfigMap
	ciliumDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultCiliumReleaseName,
			Namespace: defaultCiliumNamespace,
			Labels: map[string]string{
				"app": defaultCiliumReleaseName,
			},
			Generation: 1,
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
	ciliumDaemonSet.Status = appsv1.DaemonSetStatus{
		ObservedGeneration: 1,
	}
	Expect(remoteClient.Status().Update(ctx, ciliumDaemonSet)).To(Succeed())

	configMap = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ciliumConfigMapName,
			Namespace: defaultCiliumNamespace,
		},
		Data: map[string]string{
			kubeProxyReplacementConfigKey: "true",
		},
	}
	Expect(remoteClient.Create(ctx, configMap)).To(Succeed())

	return cluster, remoteClient
}

func disableKubeProxy(cluster *clusterv1.Cluster) error {
	spec, err := apivariables.UnmarshalClusterConfigVariable(cluster.Spec.Topology.Variables)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cluster variable: %w", err)
	}

	if spec == nil {
		spec = &apivariables.ClusterConfigSpec{}
	}
	if spec.KubeProxy == nil {
		spec.KubeProxy = &v1alpha1.KubeProxy{
			Mode: v1alpha1.KubeProxyModeDisabled,
		}
	}

	variable, err := apivariables.MarshalToClusterVariable(v1alpha1.ClusterConfigVariableName, spec)
	if err != nil {
		return fmt.Errorf("failed to marshal cluster variable: %w", err)
	}
	cluster.Spec.Topology.Variables = apivariables.UpdateClusterVariable(variable, cluster.Spec.Topology.Variables)

	return nil
}
