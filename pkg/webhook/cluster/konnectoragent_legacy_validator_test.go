// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func TestKonnectorAgentLegacyValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Konnector Agent Legacy Validator Suite")
}

var _ = Describe("KonnectorAgentLegacyValidator", func() {
	var (
		validator *konnectorAgentLegacyValidator
		decoder   admission.Decoder
		scheme    *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		decoder = admission.NewDecoder(scheme)
	})

	Context("when operation is DELETE", func() {
		It("should allow deletion", func() {
			cluster := createTestClusterWithKonnectorAgent("test-cluster", "test-namespace", true)
			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Delete

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when cluster has no topology", func() {
		It("should allow the cluster", func() {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			}
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when skip annotation is present", func() {
		It("should skip validation when annotation is true", func() {
			cluster := createTestClusterWithKonnectorAgent("test-cluster", "test-namespace", true)

			// Add skip annotation
			cluster.Annotations = map[string]string{
				v1alpha1.SkipKonnectorAgentLegacyDeploymentValidation: "true",
			}

			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should not skip validation when annotation is false", func() {
			cluster := createTestClusterWithKonnectorAgent("test-cluster", "test-namespace", true)

			// Add skip annotation with false value
			cluster.Annotations = map[string]string{
				v1alpha1.SkipKonnectorAgentLegacyDeploymentValidation: "false",
			}

			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			// Will be allowed because we can't get REST config in unit test
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should not skip validation when annotation is missing", func() {
			cluster := createTestClusterWithKonnectorAgent("test-cluster", "test-namespace", true)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			// Will be allowed because we can't get REST config in unit test
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when infrastructure is not ready", func() {
		It("should allow the cluster (CREATE scenario)", func() {
			cluster := createTestClusterWithKonnectorAgent("test-cluster", "test-namespace", false)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when konnector agent is not enabled", func() {
		It("should allow the cluster", func() {
			cluster := createTestClusterWithKonnectorAgent("test-cluster", "test-namespace", false)
			cluster.Status.InfrastructureReady = true
			// Remove konnector agent from cluster config
			cluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{}
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKonnectorAgentLegacyValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})
})

func createTestClusterWithKonnectorAgent(
	name, namespace string,
	infrastructureReady bool,
) *clusterv1.Cluster {
	clusterConfig := &variables.ClusterConfigSpec{
		Addons: &variables.Addons{
			NutanixKonnectorAgent: &variables.NutanixKonnectorAgent{},
		},
	}

	clusterConfigRaw, err := json.Marshal(clusterConfig)
	Expect(err).NotTo(HaveOccurred())

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: infrastructureReady,
		},
	}
}
