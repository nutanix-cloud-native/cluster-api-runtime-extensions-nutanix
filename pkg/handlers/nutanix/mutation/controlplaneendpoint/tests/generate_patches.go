// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
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
		// capitest.PatchTestDef{
		// 	Name: "ControlPlaneEndpoint set to valid host",
		// 	Vars: []runtimehooksv1.Variable{
		// 		capitest.VariableWithValue(
		// 			variableName,
		// 			v1alpha1.NutanixControlPlaneEndpointSpec{
		// 				Host: "10.20.100.10",
		// 				Port: 6443,
		// 			},
		// 			variablePath...,
		// 		),
		// 	},
		// 	RequestItem: request.NewNutanixClusterTemplateRequestItem("1234"),
		// 	ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
		// 		Operation: "add",
		// 		Path:      "/spec/template/spec/controlPlaneEndpoint",
		// 		ValueMatcher: gomega.HaveKeyWithValue(
		// 			"host", "10.20.100.10",
		// 		),
		// 	}},
		// },
	)
}
