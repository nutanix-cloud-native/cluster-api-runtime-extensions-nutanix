// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		MetaVariableName,
		ptr.To(v1alpha1.GenericClusterConfig{}.VariableSchema()),
		false,
		NewVariable,
		// CNI
		capitest.VariableTestDef{
			Name: "CNI: set with valid provider",
			Vals: v1alpha1.GenericClusterConfig{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "set with invalid provider",
			Vals: v1alpha1.GenericClusterConfig{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: "invalid-provider",
					},
				},
			},
			ExpectError: true,
		},
		// HTTPProxy
		capitest.VariableTestDef{
			Name: "HTTPProxy: valid proxy config",
			Vals: v1alpha1.GenericClusterConfig{
				Proxy: &v1alpha1.HTTPProxy{
					HTTP:         "http://a.b.c.example.com",
					HTTPS:        "https://a.b.c.example.com",
					AdditionalNo: []string{"d.e.f.example.com"},
				},
			},
		},
		// ExtraAPIServerCertSANs
		capitest.VariableTestDef{
			Name: "ExtraAPIServerCertSANs: single valid SAN",
			Vals: v1alpha1.GenericClusterConfig{
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{"a.b.c.example.com"},
			},
		},
		capitest.VariableTestDef{
			Name: "ExtraAPIServerCertSANs: single invalid SAN",
			Vals: v1alpha1.GenericClusterConfig{
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{"invalid:san"},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "ExtraAPIServerCertSANs: duplicate valid SANs",
			Vals: v1alpha1.GenericClusterConfig{
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{
					"a.b.c.example.com",
					"a.b.c.example.com",
				},
			},
			ExpectError: true,
		},
		// KubernetesImageRepository
		capitest.VariableTestDef{
			Name: "KubernetesImageRepository: set",
			Vals: v1alpha1.GenericClusterConfig{
				KubernetesImageRepository: ptr.To(
					v1alpha1.KubernetesImageRepository("my-registry.io/my-org/my-repo"),
				),
			},
		},
		// Etcd
		capitest.VariableTestDef{
			Name: "Etcd: unset",
			Vals: v1alpha1.GenericClusterConfig{
				Etcd: &v1alpha1.Etcd{},
			},
		},
		capitest.VariableTestDef{
			Name: "Etcd: set with valid image values",
			Vals: v1alpha1.GenericClusterConfig{
				Etcd: &v1alpha1.Etcd{
					Image: &v1alpha1.Image{
						Repository: "my-registry.io/my-org/my-repo",
						Tag:        "v3.5.99_custom.0",
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "Etcd: set with invalid image repository",
			Vals: v1alpha1.GenericClusterConfig{
				Etcd: &v1alpha1.Etcd{
					Image: &v1alpha1.Image{
						Repository: "https://this.should.not.have.a.scheme",
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "Etcd: set with invalid image tag",
			Vals: v1alpha1.GenericClusterConfig{
				Etcd: &v1alpha1.Etcd{
					Image: &v1alpha1.Image{
						Tag: "this:is:not:a:valid:tag",
					},
				},
			},
			ExpectError: true,
		},
		// ImageRegistryCredentials
		capitest.VariableTestDef{
			Name: "ImageRegistryCredentials: without a Secret",
			Vals: v1alpha1.GenericClusterConfig{
				ImageRegistries: v1alpha1.ImageRegistries{
					ImageRegistryCredentials: []v1alpha1.ImageRegistryCredentialsResource{
						{
							URL: "http://a.b.c.example.com",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "ImageRegistryCredentials: with a Secret",
			Vals: v1alpha1.GenericClusterConfig{
				ImageRegistries: v1alpha1.ImageRegistries{
					ImageRegistryCredentials: []v1alpha1.ImageRegistryCredentialsResource{
						{
							URL: "http://a.b.c.example.com",
							Secret: &corev1.ObjectReference{
								Name: "a.b.c.example.com-creds",
							},
						},
					},
				},
			},
		},
		// Combined
		capitest.VariableTestDef{
			Name: "valid config",
			Vals: v1alpha1.GenericClusterConfig{
				Proxy: &v1alpha1.HTTPProxy{
					HTTP:         "http://a.b.c.example.com",
					HTTPS:        "https://a.b.c.example.com",
					AdditionalNo: []string{"d.e.f.example.com"},
				},
				ExtraAPIServerCertSANs: v1alpha1.ExtraAPIServerCertSANs{"a.b.c.example.com"},
			},
		},
	)
}
