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

func TestCiliumValidator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cilium Validator Suite")
}

var _ = Describe("AdvancedCiliumConfigurationValidator", func() {
	var (
		validator *advancedCiliumConfigurationValidator
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
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, nil)
			req := createAdmissionRequest(cluster)
			req.Operation = admissionv1.Delete

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

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
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when skip annotation is present", func() {
		It("should skip validation when annotation is true", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)

			// Add skip annotation
			cluster.Annotations = map[string]string{
				v1alpha1.SkipCiliumKubeProxyReplacementValidation: "true",
			}

			req := createAdmissionRequest(cluster)

			// Create ConfigMap with kubeProxyReplacement set to false - should still allow due to annotation
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"values.yaml": `
ipam:
  mode: kubernetes
kubeProxyReplacement: false
`,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should not skip validation when annotation is false", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)

			// Add skip annotation with false value
			cluster.Annotations = map[string]string{
				v1alpha1.SkipCiliumKubeProxyReplacementValidation: "false",
			}

			req := createAdmissionRequest(cluster)

			// Create ConfigMap with kubeProxyReplacement set to false - should deny
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"values.yaml": `
ipam:
  mode: kubernetes
kubeProxyReplacement: false
`,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
		})

		It("should not skip validation when annotation is missing", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			// Create ConfigMap with kubeProxyReplacement set to false - should deny
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"values.yaml": `
ipam:
  mode: kubernetes
kubeProxyReplacement: false
`,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
		})
	})

	Context("when kube-proxy is not disabled", func() {
		It("should allow the cluster", func() {
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeIPTables, nil)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("when kube-proxy is disabled", func() {
		It("should allow when CNI is not configured", func() {
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, nil)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should allow when CNI provider is not Cilium", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCalico,
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should allow when Cilium is configured with default values", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should allow when ConfigMap does not exist", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should allow when ConfigMap does not have values.yaml key", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			// Create ConfigMap without values.yaml key
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"some-key": "some-value",
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})

		It("should deny when ConfigMap does not have kubeProxyReplacement enabled", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			// Create ConfigMap without kubeProxyReplacement
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"values.yaml": `
ipam:
  mode: kubernetes
`,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("does not have 'kubeProxyReplacement' enabled"))
		})

		It("should deny when kubeProxyReplacement is set to false", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			// Create ConfigMap with kubeProxyReplacement set to false
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"values.yaml": `
ipam:
  mode: kubernetes
kubeProxyReplacement: false
`,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("does not have 'kubeProxyReplacement' enabled"))
		})

		It("should allow when kubeProxyReplacement is set to true", func() {
			cni := &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCilium,
				AddonConfig: v1alpha1.AddonConfig{
					Values: &v1alpha1.AddonValues{
						SourceRef: &v1alpha1.ValuesReference{
							Kind: "ConfigMap",
							Name: "cilium-values",
						},
					},
				},
			}
			cluster := createTestCluster("test-cluster", "test-namespace", v1alpha1.KubeProxyModeDisabled, cni)
			req := createAdmissionRequest(cluster)

			// Create ConfigMap with kubeProxyReplacement set to true
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cilium-values",
					Namespace: "test-namespace",
				},
				Data: map[string]string{
					"values.yaml": `
ipam:
  mode: kubernetes
kubeProxyReplacement: true
`,
				},
			}

			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(configMap).Build()
			validator = NewAdvancedCiliumConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})
})

func createTestCluster(
	name, namespace string,
	kubeProxyMode v1alpha1.KubeProxyMode,
	cni *v1alpha1.CNI,
) *clusterv1.Cluster {
	clusterConfig := &variables.ClusterConfigSpec{
		KubeProxy: &v1alpha1.KubeProxy{
			Mode: kubeProxyMode,
		},
	}

	if cni != nil {
		clusterConfig.Addons = &variables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CNI: cni,
			},
		}
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
	}
}

func createAdmissionRequest(cluster *clusterv1.Cluster) admission.Request {
	objRaw, err := json.Marshal(cluster)
	Expect(err).NotTo(HaveOccurred())

	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Raw: objRaw,
			},
			RequestKind: &metav1.GroupVersionKind{
				Group:   clusterv1.GroupVersion.Group,
				Version: clusterv1.GroupVersion.Version,
				Kind:    "Cluster",
			},
		},
	}
}
