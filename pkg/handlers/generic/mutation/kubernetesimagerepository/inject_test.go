// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package kubernetesimagerepository

import (
	"testing"

	. "github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

func TestGeneratePatches(t *testing.T) {
	capitest.ValidateGeneratePatches(
		t,
		func() mutation.GeneratePatches { return NewPatch() },
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "kubernetesImageRepository set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.KubernetesImageRepository("my-registry.io/my-org/my-repo"),
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation: "add",
				Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
				ValueMatcher: HaveKeyWithValue(
					"imageRepository",
					"my-registry.io/my-org/my-repo",
				),
			}},
		},
	)
}
