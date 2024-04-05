// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
)

func TestGeneratePatches(
	t GinkgoTInterface,
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
			Name: "region set",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					"a-specific-region",
					variablePath...,
				),
			},
			RequestItem: request.NewAWSClusterTemplateRequestItem("1234"),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{{
				Operation:    "add",
				Path:         "/spec/template/spec/region",
				ValueMatcher: gomega.Equal("a-specific-region"),
			}},
		},
	)
}
