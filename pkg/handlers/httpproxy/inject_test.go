// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiserver/pkg/storage/names"
	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestGeneratePatches(t *testing.T) {
	capitest.ValidateGeneratePatches(
		t,
		func() *httpProxyPatchHandler {
			fakeClient := fake.NewClientBuilder().Build()
			return NewPatch(fakeClient)
		},
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "http proxy set for KubeadmConfigTemplate default-worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": "default-worker",
						},
					},
				),
			},
			RequestItem: capitest.NewKubeadmConfigTemplateRequestItem(),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/files",
				ValueMatcher: HaveLen(2),
			}},
		},
		capitest.PatchTestDef{
			Name: "http proxy set for KubeadmConfigTemplate generic worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
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
			RequestItem: capitest.NewKubeadmConfigTemplateRequestItem(),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/files",
				ValueMatcher: HaveLen(2),
			}},
		},
		capitest.PatchTestDef{
			Name: "http proxy set for KubeadmControlPlaneTemplate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.HTTPProxy{
						HTTP:         "http://example.com",
						HTTPS:        "https://example.com",
						AdditionalNo: []string{"no-proxy.example.com"},
					},
				),
			},
			RequestItem: capitest.NewKubeadmControlPlaneTemplateRequestItem(),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/kubeadmConfigSpec/files",
				ValueMatcher: HaveLen(2),
			}},
		},
	)
}

func TestGenerateNoProxy(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	testCases := []struct {
		name            string
		cluster         *capiv1.Cluster
		expectedNoProxy []string
	}{
		{
			name:    "no networking config",
			cluster: &capiv1.Cluster{},
			expectedNoProxy: []string{
				"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
				".svc", ".svc.cluster.local",
			},
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()
			g.Expect(generateNoProxy(tc.cluster)).To(Equal(tc.expectedNoProxy))
		})
	}
}
