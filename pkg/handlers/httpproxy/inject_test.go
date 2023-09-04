// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/storage/names"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestGeneratePatches(t *testing.T) {
	capitest.ValidateGeneratePatches(
		t,
		NewPatch,
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "http proxy set for KubeadmConfigTemplate default-worker",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					VariableName,
					HTTPProxyVariables{
						HTTP:  "http://example.com",
						HTTPS: "https://example.com",
						No:    []string{"no-proxy.example.com"},
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
					VariableName,
					HTTPProxyVariables{
						HTTP:  "http://example.com",
						HTTPS: "https://example.com",
						No:    []string{"no-proxy.example.com"},
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
					VariableName,
					HTTPProxyVariables{
						HTTP:  "http://example.com",
						HTTPS: "https://example.com",
						No:    []string{"no-proxy.example.com"},
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
