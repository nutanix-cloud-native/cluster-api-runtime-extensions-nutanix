// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

var _ = Describe("ImageRegistryValidator", func() {
	var (
		validator *imageRegistryValidator
		decoder   admission.Decoder
		scheme    *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(clusterv1beta2.AddToScheme(scheme)).To(Succeed())
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())
		decoder = admission.NewDecoder(scheme)
	})

	Context("same host with http and https should be rejected", func() {
		It("should reject", func() {
			clusterConfig := &variables.ClusterConfigSpec{
				GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
					ImageRegistries: []v1alpha1.ImageRegistry{
						{URL: "http://myregistry.com"},
						{URL: "https://myregistry.com"},
					},
				},
			}
			cluster := createClusterWithGenericConfig(clusterConfig)
			req := createImageRegistryAdmissionRequest(cluster)
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewImageRegistryValidator(client, decoder)
			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Message).To(ContainSubstring("conflicting scheme for host"))
		})
	})

	Context("same host with same http scheme should be allowed", func() {
		It("should allow", func() {
			clusterConfig := &variables.ClusterConfigSpec{
				GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
					ImageRegistries: []v1alpha1.ImageRegistry{
						{URL: "http://myregistry.com/library"},
						{URL: "http://myregistry.com/app-images"},
					},
				},
			}
			cluster := createClusterWithGenericConfig(clusterConfig)
			req := createImageRegistryAdmissionRequest(cluster)
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewImageRegistryValidator(client, decoder)
			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("different hosts with http and https should be allowed", func() {
		It("should allow", func() {
			clusterConfig := &variables.ClusterConfigSpec{
				GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
					ImageRegistries: []v1alpha1.ImageRegistry{
						{URL: "http://myregistry.com"},
						{URL: "https://other-registry.com"},
					},
				},
			}
			cluster := createClusterWithGenericConfig(clusterConfig)
			req := createImageRegistryAdmissionRequest(cluster)
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewImageRegistryValidator(client, decoder)
			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})

	Context("global mirror http and imageRegistry https same host should be allowed", func() {
		It("should allow", func() {
			clusterConfig := &variables.ClusterConfigSpec{
				GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
					GlobalImageRegistryMirror: &v1alpha1.GlobalImageRegistryMirror{
						URL: "http://myregistry.com/library",
					},
					ImageRegistries: []v1alpha1.ImageRegistry{
						{URL: "https://myregistry.com/app-images"},
					},
				},
			}
			cluster := createClusterWithGenericConfig(clusterConfig)
			req := createImageRegistryAdmissionRequest(cluster)
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator = NewImageRegistryValidator(client, decoder)
			resp := validator.validate(context.Background(), req)
			Expect(resp.Allowed).To(BeTrue())
		})
	})
})

func createClusterWithGenericConfig(cfg *variables.ClusterConfigSpec) *clusterv1beta2.Cluster {
	cfgRaw, err := json.Marshal(cfg)
	Expect(err).NotTo(HaveOccurred())
	return &clusterv1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				ClassRef: clusterv1beta2.ClusterClassRef{Name: "test-class"},
				Version:  "v1.30.0",
				Variables: []clusterv1beta2.ClusterVariable{
					{
						Name:  v1alpha1.ClusterConfigVariableName,
						Value: apiextensionsv1.JSON{Raw: cfgRaw},
					},
				},
			},
		},
	}
}

func createImageRegistryAdmissionRequest(cluster *clusterv1beta2.Cluster) admission.Request {
	return createKubeletAdmissionRequest(cluster)
}
