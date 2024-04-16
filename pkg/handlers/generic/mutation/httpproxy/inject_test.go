// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestGenerateNoProxy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cluster         *clusterv1.Cluster
		expectedNoProxy []string
	}{{
		name:    "no networking config",
		cluster: &clusterv1.Cluster{},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local",
		},
	}, {
		name: "custom pod network",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Pods: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"10.0.0.0/24", "10.0.1.0/24"},
					},
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "10.0.0.0/24", "10.0.1.0/24", "kubernetes",
			"kubernetes.default", ".svc", ".svc.cluster.local",
		},
	}, {
		name: "Unknown infrastructure cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "SomeFakeInfrastructureCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local",
		},
	}, {
		name: "AWS cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AWSCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", "169.254.169.254", ".elb.amazonaws.com",
		},
	}, {
		name: "AWS managed (EKS) cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AWSManagedCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", "169.254.169.254", ".elb.amazonaws.com",
		},
	}, {
		name: "Azure cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AzureCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", "169.254.169.254",
		},
	}, {
		name: "Azure managed (AKS) cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AzureCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", "169.254.169.254",
		},
	}, {
		name: "GCP cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "GCPCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", "169.254.169.254", "metadata", "metadata.google.internal",
		},
	}, {
		name: "custom service network",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Services: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/24", "172.16.1.0/24"},
					},
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "172.16.0.0/24", "172.16.1.0/24", "kubernetes",
			"kubernetes.default", ".svc", ".svc.cluster.local",
		},
	}, {
		name: "custom servicedomain",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					ServiceDomain: "foo.bar",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.foo.bar",
		},
	}, {
		name: "all options",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Pods: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"10.10.0.0/16"},
					},
					Services: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/16"},
					},
					ServiceDomain: "foo.bar",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "10.10.0.0/16", "172.16.0.0/16", "kubernetes",
			"kubernetes.default", ".svc", ".svc.foo.bar",
		},
	}}

	for idx := range testCases {
		tt := testCases[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			g.Expect(generateNoProxy(tt.cluster)).To(gomega.Equal(tt.expectedNoProxy))
		})
	}
}

func TestHTTPProxyPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Proxy mutator suite")
}

var _ = Describe("Generate HTTPProxy Patches", func() {
	// only add HTTPProxy patch
	patchGenerator := func() mutation.GeneratePatches {
		// Always initialize the testEnv variable in the closure.
		// This will allow ginkgo to initialize testEnv variable during test execution time.
		clientScheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
		utilruntime.Must(clusterv1.AddToScheme(clientScheme))
		cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			cl,
			NewPatch(cl)).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "http proxy set for KubeadmConfigTemplate generic worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
					VariableName,
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/files",
				ValueMatcher: gomega.ContainElements(
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/containerd.service.d/http-proxy.conf",
					),
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/kubelet.service.d/http-proxy.conf",
					),
				),
			}},
		},
		{
			Name: "http proxy set for KubeadmControlPlaneTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: gomega.ContainElements(
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/containerd.service.d/http-proxy.conf",
					),
					gomega.HaveKeyWithValue(
						"path", "/etc/systemd/system/kubelet.service.d/http-proxy.conf",
					),
				),
			}},
		},
	}
	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			clientScheme := runtime.NewScheme()
			utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
			utilruntime.Must(clusterv1.AddToScheme(clientScheme))
			cl, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			gomega.Expect(err).To(gomega.BeNil())
			c := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
			}
			err = cl.Create(context.Background(), c)
			gomega.Expect(err).To(gomega.BeNil())
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
			err = cl.Delete(context.Background(), c)
			gomega.Expect(err).To(gomega.BeNil())
		})
	}
})
