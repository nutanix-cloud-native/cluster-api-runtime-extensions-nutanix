// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

var _ = Describe("KonnectorAgentLegacyValidator", Serial, func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	decoder := admission.NewDecoder(clientScheme)

	Context("when operation is not UPDATE", func() {
		It("should skip validation with CREATE operation", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Create

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should skip validation with DELETE operation", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Delete

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when cluster has no topology", func() {
		It("should skip validation for the cluster", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-cluster-",
					Namespace:    corev1.NamespaceDefault,
				},
			}
			Expect(c.Create(ctx, cluster)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when skip annotation is present", func() {
		It("should skip validation when annotation is true", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			// Add skip annotation
			cluster.Annotations = map[string]string{
				v1alpha1.SkipKonnectorAgentLegacyDeploymentValidation: "true",
			}
			Expect(c.Create(ctx, cluster)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should run validation when annotation is false", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			// Add skip annotation with false value
			cluster.Annotations = map[string]string{
				v1alpha1.SkipKonnectorAgentLegacyDeploymentValidation: "false",
			}
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = true
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeTrue())
		})

		It("should run validation when annotation is missing", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = true
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when infrastructure is not ready", func() {
		It("should skip validation even with UPDATE operation", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = false
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-namespace-"}}
			Expect(c.Create(ctx, ns)).To(Succeed())
			cm := createLegacyAgentConfigMap(ns)
			Expect(c.Create(ctx, cm)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeTrue())

			// The Kubernetes client is shared across tests, delete the test ConfigMap to avoid polluting other cases.
			cleanupTestConfigMap(ctx, c, ns)
		})
	})

	Context("when konnector agent is not enabled", func() {
		It("should skip validation", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			// Remove konnector agent from cluster config
			cluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{}
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = false
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-namespace-"}}
			Expect(c.Create(ctx, ns)).To(Succeed())
			cm := createLegacyAgentConfigMap(ns)
			Expect(c.Create(ctx, cm)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeTrue())

			// The Kubernetes client is shared across tests, delete the test ConfigMap to avoid polluting other cases.
			cleanupTestConfigMap(ctx, c, ns)
		})
	})

	Context("when agent ConfigMap exists in the cluster", func() {
		It("should allow the cluster when ConfigMap does not have Helm annotations", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = true
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-namespace-"}}
			Expect(c.Create(ctx, ns)).To(Succeed())
			cm := createAgentConfigMapWithNoAnnotations(ns)
			Expect(c.Create(ctx, cm)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeTrue())

			// The Kubernetes client is shared across tests, delete the test ConfigMap to avoid polluting other cases.
			cleanupTestConfigMap(ctx, c, ns)
		})

		It("should allow the cluster when ConfigMap has new Konnector agent Helm annotations", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = true
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-namespace-"}}
			Expect(c.Create(ctx, ns)).To(Succeed())
			cm := createKonnectorAgentConfigMap(ns)
			Expect(c.Create(ctx, cm)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeTrue())

			// The Kubernetes client is shared across tests, delete the test ConfigMap to avoid polluting other cases.
			cleanupTestConfigMap(ctx, c, ns)
		})

		It("should not allow the cluster when ConfigMap has legacy agent Helm annotations", func() {
			c, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			cluster := createTestClusterWithKonnectorAgent()
			Expect(c.Create(ctx, cluster)).To(Succeed())
			cluster.Status.InfrastructureReady = true
			Expect(c.Status().Update(ctx, cluster)).To(Succeed())
			Expect(helpers.TestEnv.WithFakeRemoteClusterClient(cluster)).To(Succeed())

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "test-namespace-"}}
			Expect(c.Create(ctx, ns)).To(Succeed())
			cm := createLegacyAgentConfigMap(ns)
			Expect(c.Create(ctx, cm)).To(Succeed())

			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Update

			validator := NewKonnectorAgentLegacyValidator(c, decoder)

			resp := validator.validate(context.Background(), req)

			Expect(resp.Allowed).To(BeFalse())
			errMsg := fmt.Sprintf(`
Cannot enable onboarding functionality as an addon: legacy installation(s) detected.

Found 1 release(s) for chart "nutanix-k8s-agent": [legacy-agent (namespace: %s)] in the cluster.

ACTION REQUIRED: Uninstall the legacy Helm release(s) before proceeding to avoid conflicts.

To uninstall, run the following command(s):
  helm uninstall legacy-agent -n %s --kubeconfig <kubeconfig-path>

If the release is stuck or uninstall fails, use the force removal command:
  helm uninstall legacy-agent -n %s --no-hooks --kubeconfig <kubeconfig-path>

After removing the legacy release(s), re-run the operation.`, ns.Name, ns.Name, ns.Name)
			Expect(resp.Result.Message).To(Equal(errMsg))

			// The Kubernetes client is shared across tests, delete the test ConfigMap to avoid polluting other cases.
			cleanupTestConfigMap(ctx, c, ns)
		})
	})
})

// createTestClusterWithKonnectorAgent creates a test Cluster and a corresponding kubeconfig Secret.
func createTestClusterWithKonnectorAgent() *clusterv1.Cluster {
	clusterConfig := &variables.ClusterConfigSpec{
		Addons: &variables.Addons{
			NutanixKonnectorAgent: &variables.NutanixKonnectorAgent{},
		},
	}

	clusterConfigRaw, err := json.Marshal(clusterConfig)
	Expect(err).NotTo(HaveOccurred())

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    corev1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Class:   "test-class",
				Version: "v1.30.0",
				Variables: []clusterv1.ClusterVariable{
					{
						Name: v1alpha1.ClusterConfigVariableName,
						Value: apiextensionsv1.JSON{
							Raw: clusterConfigRaw,
						},
					},
				},
			},
		},
	}
}

func createAgentConfigMapWithNoAnnotations(namespace *corev1.Namespace) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentConfigMapName,
			Namespace: namespace.Name,
		},
	}
}

func createLegacyAgentConfigMap(namespace *corev1.Namespace) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentConfigMapName,
			Namespace: namespace.Name,
			Annotations: map[string]string{
				releaseNameAnnotation:      "legacy-agent",
				releaseNamespaceAnnotation: namespace.Name,
			},
		},
	}
}

func createKonnectorAgentConfigMap(namespace *corev1.Namespace) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentConfigMapName,
			Namespace: namespace.Name,
			Annotations: map[string]string{
				releaseNameAnnotation:      konnectorAgentReleaseName,
				releaseNamespaceAnnotation: konnectorAgentReleaseNamespace,
			},
		},
	}
}

// cleanupConfigMap deletes the ConfigMap and waits for it to be fully deleted.
// This ensures that ConfigMaps from previous tests don't interfere with subsequent tests,
// since findLegacyReleases searches across all namespaces.
func cleanupTestConfigMap(ctx context.Context, c client.Client, namespace *corev1.Namespace) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentConfigMapName,
			Namespace: namespace.Name,
		},
	}

	// Delete the ConfigMap (ignore if it doesn't exist)
	if err := c.Delete(ctx, configMap); err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	// Wait for the ConfigMap to be fully deleted
	Eventually(func() bool {
		cm := &corev1.ConfigMap{}
		err := c.Get(ctx, client.ObjectKey{Name: agentConfigMapName, Namespace: namespace.Name}, cm)
		return apierrors.IsNotFound(err)
	}).WithTimeout(30 * time.Second).WithPolling(100 * time.Millisecond).Should(BeTrue())
}
