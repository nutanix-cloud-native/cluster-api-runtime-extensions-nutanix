// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"testing"

	. "github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestGeneratePatches(t *testing.T) {
	capitest.ValidateGeneratePatches(
		t,
		func() mutation.GeneratePatches { return NewPatch() },
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "etcd imageRepository and imageTag set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Repository: "my-registry.io/my-org/my-repo",
							Tag:        "v3.5.99_custom.0",
						},
					},
				),
			},
			RequestItem: capitest.NewKubeadmControlPlaneTemplateRequestItem(),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: HaveKeyWithValue(
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
		capitest.PatchTestDef{
			Name: "etcd imageRepository set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Repository: "my-registry.io/my-org/my-repo",
						},
					},
				),
			},
			RequestItem: capitest.NewKubeadmControlPlaneTemplateRequestItem(),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: HaveKeyWithValue(
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
		capitest.PatchTestDef{
			Name: "etcd imageTag set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.Etcd{
						Image: &v1alpha1.Image{
							Tag: "v3.5.99_custom.0",
						},
					},
				),
			},
			RequestItem: capitest.NewKubeadmControlPlaneTemplateRequestItem(),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
					ValueMatcher: HaveKeyWithValue(
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
	)
}
