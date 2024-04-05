// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestEtcdPolicyPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "etcd mutator suite")
}

var _ = Describe("Generate etcd patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch()).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "unset variable",
		},
		{
			Name: "etcd imageRepository and imageTag set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Repository: "my-registry.io/my-org/my-repo",
							Tag:        "v3.5.99_custom.0",
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"etcd",
						map[string]interface{}{
							"local": map[string]interface{}{
								"imageRepository": "my-registry.io/my-org/my-repo",
								"imageTag":        "v3.5.99_custom.0",
							},
						},
					),
				},
			},
		},
		{
			Name: "etcd imageRepository set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Repository: "my-registry.io/my-org/my-repo",
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"etcd",
						map[string]interface{}{
							"local": map[string]interface{}{
								"imageRepository": "my-registry.io/my-org/my-repo",
							},
						},
					),
				},
			},
		},
		{
			Name: "etcd imageTag set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Tag: "v3.5.99_custom.0",
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: gomega.HaveKeyWithValue(
						"etcd",
						map[string]interface{}{
							"local": map[string]interface{}{
								"imageTag": "v3.5.99_custom.0",
							},
						},
					),
				},
			},
		},
	}

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})
