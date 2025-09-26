// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package externalcloudprovider

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

func TestExternalCloudProviderPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "External cloud provider mutator suite")
}

var _ = Describe("Generate external cloud provider patches", func() {
	// only add aws region patch
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", helpers.TestEnv.Client, NewControlPlanePatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name:            "no kubernetes version available",
			RequestItem:     request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedFailure: true,
		},
		{
			Name:            "explicit empty kubernetes version specified",
			RequestItem:     (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).WithKubernetesVersion("").NewRequest(""),
			ExpectedFailure: true,
		},
		{
			Name: "non-semver kubernetes version specified",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithKubernetesVersion("this is not semver").NewRequest(""),
			ExpectedFailure: true,
		},
		{
			Name: "add API server flag for pre-1.33.0 control plane",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithKubernetesVersion("1.32.29").NewRequest(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"apiServer",
						gomega.Equal(map[string]interface{}{
							"extraArgs": map[string]interface{}{
								"cloud-provider": "external",
							},
						}),
					),
				},
			},
		},
		{
			Name: "add API server flag for pre-1.33.0 control plane with pre-existing extra args",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithKubernetesVersion("1.32.29").
				WithAPIServerExtraArgs(map[string]string{"foo": "bar"}).
				NewRequest(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/apiServer/extraArgs/cloud-provider",
					ValueMatcher: gomega.Equal("external"),
				},
			},
		},
		{
			Name: "no patches added for >= 1.33.0 control plane",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithKubernetesVersion("1.33.0").
				WithAPIServerExtraArgs(map[string]string{"foo": "bar"}).
				NewRequest(""),
		},
		{
			Name: "no patches added for >= 1.33.0 control plane take 2",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithKubernetesVersion("2.72.0").
				WithAPIServerExtraArgs(map[string]string{"foo": "bar"}).
				NewRequest(""),
		},
		{
			Name: "no patches added for 1.33.0 pre-release control plane",
			RequestItem: (&request.KubeadmControlPlaneTemplateRequestItemBuilder{}).
				WithKubernetesVersion("1.33.0-alpha.1").
				WithAPIServerExtraArgs(map[string]string{"foo": "bar"}).
				NewRequest(""),
		},
	}

	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(
				GinkgoT(),
				patchGenerator,
				&tt,
			)
		})
	}
})
