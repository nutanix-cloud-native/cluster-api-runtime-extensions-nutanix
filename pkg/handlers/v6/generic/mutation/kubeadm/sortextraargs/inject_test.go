// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sortextraargs

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestSortExtraArgsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Sort extra args mutator suite")
}

var _ = Describe("Generate sort extra args patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:        "no extra args - no patches generated",
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
		},
		{
			Name: "unsorted apiServer extraArgs are sorted",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: "zzz", Value: ptr.To("last")},
					{Name: "aaa", Value: ptr.To("first")},
					{Name: "mmm", Value: ptr.To("middle")},
				}).
				NewRequest(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/0/name",
					ValueMatcher: gomega.Equal("aaa"),
				},
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/1/name",
					ValueMatcher: gomega.Equal("mmm"),
				},
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/2/name",
					ValueMatcher: gomega.Equal("zzz"),
				},
			},
		},
		{
			Name: "already sorted apiServer extraArgs produce no patches",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithAPIServerExtraArgs([]bootstrapv1.Arg{
					{Name: "aaa", Value: ptr.To("first")},
				}).
				NewRequest(""),
		},
		{
			Name: "unsorted kubelet extraArgs are sorted for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					runtimehooksv1.BuiltinsName,
					map[string]any{
						"machineDeployment": map[string]any{
							"class": "default-worker",
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItemWithKubeletExtraArgs([]bootstrapv1.Arg{
				{Name: "zzz", Value: ptr.To("last")},
				{Name: "aaa", Value: ptr.To("first")},
			}),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/0/name",
					ValueMatcher: gomega.Equal("aaa"),
				},
				{
					Operation:    "replace",
					Path:         "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/1/name",
					ValueMatcher: gomega.Equal("zzz"),
				},
			},
		},
	}

	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
