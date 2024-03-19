// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestControlPlaneGeneratePatches(
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
			Name: "image unset for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{Raw: []byte(`{"controlPlane": {"version": "v1.2.3"}}`)},
				),
			},
			RequestItem: request.NewCPDockerMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/customImage",
				ValueMatcher: gomega.Equal("ghcr.io/mesosphere/kind-node:v1.2.3"),
			}},
		},
		capitest.PatchTestDef{
			Name: "image set for control plane",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					"a-specific-image",
					variablePath...,
				),
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewCPDockerMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/customImage",
				ValueMatcher: gomega.Equal("a-specific-image"),
			}},
		},
	)
}

func TestWorkerGeneratePatches(
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
			Name: "image unset for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(
							`{"machineDeployment": {"class": "a-worker", "version": "v1.2.3"}}`,
						),
					},
				),
			},
			RequestItem: request.NewWorkerDockerMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/customImage",
				ValueMatcher: gomega.Equal("ghcr.io/mesosphere/kind-node:v1.2.3"),
			}},
		},
		capitest.PatchTestDef{
			Name: "image set for workers",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					"a-specific-image",
					variablePath...,
				),
				capitest.VariableWithValue(
					"builtin",
					apiextensionsv1.JSON{
						Raw: []byte(`{"machineDeployment": {"class": "a-worker"}}`),
					},
				),
			},
			RequestItem: request.NewWorkerDockerMachineTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/customImage",
				ValueMatcher: gomega.Equal("a-specific-image"),
			}},
		},
	)
}
