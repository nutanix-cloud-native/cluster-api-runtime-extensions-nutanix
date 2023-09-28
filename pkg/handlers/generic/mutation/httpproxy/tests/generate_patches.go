// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apiserver/pkg/storage/names"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

func TestGeneratePatches(
	t *testing.T,
	generatorFunc func() mutation.GeneratePatches,
	variableName string,
	variablePath ...string,
) {
	t.Helper()

	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
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
					variablePath...,
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
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/files",
				ValueMatcher: gomega.HaveLen(2),
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
					variablePath...,
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
				Operation:    "add",
				Path:         "/spec/template/spec/files",
				ValueMatcher: gomega.HaveLen(2),
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
					variablePath...,
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
	)
}
