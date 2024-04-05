// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/storage/names"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestGenerateNoProxy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cluster         *capiv1.Cluster
		expectedNoProxy []string
	}{{
		name:    "no networking config",
		cluster: &capiv1.Cluster{},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local",
		},
	}, {
		name: "custom pod network",
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
				ClusterNetwork: &capiv1.ClusterNetwork{
					Pods: &capiv1.NetworkRanges{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
				ClusterNetwork: &capiv1.ClusterNetwork{
					Services: &capiv1.NetworkRanges{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
				ClusterNetwork: &capiv1.ClusterNetwork{
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
		cluster: &capiv1.Cluster{
			Spec: capiv1.ClusterSpec{
				ClusterNetwork: &capiv1.ClusterNetwork{
					Pods: &capiv1.NetworkRanges{
						CIDRBlocks: []string{"10.10.0.0/16"},
					},
					Services: &capiv1.NetworkRanges{
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
		testEnv := helpers.TestEnv
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			NewPatch(testEnv.Client)).(mutation.GeneratePatches)
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
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})
