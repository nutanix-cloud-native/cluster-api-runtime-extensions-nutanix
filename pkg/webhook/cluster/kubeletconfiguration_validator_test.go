// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

var _ = Describe("KubeletConfigurationValidator", func() {
	var (
		validator *kubeletConfigurationValidator
		decoder   admission.Decoder
		scheme    *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		decoder = admission.NewDecoder(scheme)
	})

	Context("cpuManagerPolicy=static without CPU reservation", func() {
		It("should reject", func() {
			cfg := &v1alpha1.KubeletConfiguration{
				CPUManagerPolicy: ptrOf(v1alpha1.CPUManagerPolicyStatic),
			}
			cluster := createClusterWithKubeletConfig(cfg)
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring(
				"cpuManagerPolicy 'static' requires CPU reservation in systemReserved or kubeReserved",
			))
		})
	})

	Context("cpuManagerPolicy=static with CPU in systemReserved", func() {
		It("should accept", func() {
			cfg := &v1alpha1.KubeletConfiguration{
				CPUManagerPolicy: ptrOf(v1alpha1.CPUManagerPolicyStatic),
				SystemReserved: map[string]resource.Quantity{
					"cpu": resource.MustParse("100m"),
				},
			}
			cluster := createClusterWithKubeletConfig(cfg)
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("cpuManagerPolicy=static with CPU in kubeReserved", func() {
		It("should accept", func() {
			cfg := &v1alpha1.KubeletConfiguration{
				CPUManagerPolicy: ptrOf(v1alpha1.CPUManagerPolicyStatic),
				KubeReserved: map[string]resource.Quantity{
					"cpu": resource.MustParse("500m"),
				},
			}
			cluster := createClusterWithKubeletConfig(cfg)
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("evictionHard with invalid value", func() {
		It("should reject", func() {
			cfg := &v1alpha1.KubeletConfiguration{
				EvictionHard: map[string]string{
					"memory.available": "invalid",
				},
			}
			cluster := createClusterWithKubeletConfig(cfg)
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("invalid eviction threshold value"))
		})
	})

	Context("evictionHard with valid percentage", func() {
		It("should accept", func() {
			cfg := &v1alpha1.KubeletConfiguration{
				EvictionHard: map[string]string{
					"memory.available": "10%",
				},
			}
			cluster := createClusterWithKubeletConfig(cfg)
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("evictionHard with valid quantity", func() {
		It("should accept", func() {
			cfg := &v1alpha1.KubeletConfiguration{
				EvictionHard: map[string]string{
					"memory.available": "100Mi",
				},
			}
			cluster := createClusterWithKubeletConfig(cfg)
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("both maxParallelImagePullsPerNode and kubeletConfiguration.maxParallelImagePulls set", func() {
		It("should allow with warning", func() {
			clusterConfig := &variables.ClusterConfigSpec{
				KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
					MaxParallelImagePullsPerNode: ptr.To(int32(4)), //nolint:staticcheck // testing deprecated field
					KubeletConfiguration: &v1alpha1.KubeletConfiguration{
						MaxParallelImagePulls: ptr.To(int32(8)),
					},
				},
			}
			clusterConfigRaw, err := json.Marshal(clusterConfig)
			Expect(err).NotTo(HaveOccurred())

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "test-class",
						Version: "v1.30.0",
						Variables: []clusterv1.ClusterVariable{
							{
								Name:  v1alpha1.ClusterConfigVariableName,
								Value: apiextensionsv1.JSON{Raw: clusterConfigRaw},
							},
						},
					},
				},
			}
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
			Expect(resp.Warnings).To(ContainElement(
				ContainSubstring("maxParallelImagePullsPerNode will be ignored"),
			))
		})
	})

	Context("cpuManagerPolicy=static in control plane override without CPU reservation", func() {
		It("should reject", func() {
			clusterConfig := &variables.ClusterConfigSpec{
				ControlPlane: &variables.ControlPlaneSpec{
					KubeadmNodeSpec: v1alpha1.KubeadmNodeSpec{
						KubeletConfiguration: &v1alpha1.KubeletConfiguration{
							CPUManagerPolicy: ptrOf(v1alpha1.CPUManagerPolicyStatic),
						},
					},
				},
			}
			clusterConfigRaw, err := json.Marshal(clusterConfig)
			Expect(err).NotTo(HaveOccurred())

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{
						Class:   "test-class",
						Version: "v1.30.0",
						Variables: []clusterv1.ClusterVariable{
							{
								Name:  v1alpha1.ClusterConfigVariableName,
								Value: apiextensionsv1.JSON{Raw: clusterConfigRaw},
							},
						},
					},
				},
			}
			req := createKubeletAdmissionRequest(cluster)

			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewKubeletConfigurationValidator(client, decoder)

			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring(
				"clusterConfig.controlPlane.kubeletConfiguration",
			))
		})
	})
})

func ptrOf[T any](v T) *T {
	return &v
}

func createClusterWithKubeletConfig(cfg *v1alpha1.KubeletConfiguration) *clusterv1.Cluster {
	clusterConfig := &variables.ClusterConfigSpec{
		KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
			KubeletConfiguration: cfg,
		},
	}

	clusterConfigRaw, err := json.Marshal(clusterConfig)
	Expect(err).NotTo(HaveOccurred())

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
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

func createKubeletAdmissionRequest(cluster *clusterv1.Cluster) admission.Request {
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
