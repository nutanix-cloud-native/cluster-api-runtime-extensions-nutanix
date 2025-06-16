// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubeproxymode

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestKubeProxyModePatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "kube-proxy mode mutator suite")
}

type testObj struct {
	patchTest capitest.PatchTestDef
}

var _ = Describe("Generate kube proxy mode patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", nil, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []testObj{{
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with AWS",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSClusterConfigSpec{
						GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
							KubeProxy: &v1alpha1.KubeProxy{
								Mode: v1alpha1.KubeProxyModeDisabled,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/skipPhases",
				ValueMatcher: gomega.ConsistOf([]string{
					"addon/kube-proxy",
				}),
			}},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with Docker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.DockerClusterConfigSpec{
						GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
							KubeProxy: &v1alpha1.KubeProxy{
								Mode: v1alpha1.KubeProxyModeDisabled,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/skipPhases",
				ValueMatcher: gomega.ConsistOf([]string{
					"addon/kube-proxy",
				}),
			}},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "disable kube proxy with Nutanix",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NutanixClusterConfigSpec{
						GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
							KubeProxy: &v1alpha1.KubeProxy{
								Mode: v1alpha1.KubeProxyModeDisabled,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/skipPhases",
				ValueMatcher: gomega.ConsistOf([]string{
					"addon/kube-proxy",
				}),
			}},
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy iptables mode with AWS",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.AWSClusterConfigSpec{
						GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
							KubeProxy: &v1alpha1.KubeProxy{
								Mode: v1alpha1.KubeProxyModeIPTables,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy iptables mode with Docker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.DockerClusterConfigSpec{
						GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
							KubeProxy: &v1alpha1.KubeProxy{
								Mode: v1alpha1.KubeProxyModeIPTables,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		},
	}, {
		patchTest: capitest.PatchTestDef{
			Name: "kube proxy iptables mode with Nutanix",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.NutanixClusterConfigSpec{
						GenericClusterConfigSpec: v1alpha1.GenericClusterConfigSpec{
							KubeProxy: &v1alpha1.KubeProxy{
								Mode: v1alpha1.KubeProxyModeIPTables,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		},
	}}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.patchTest.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.patchTest)
		})
	}
})
