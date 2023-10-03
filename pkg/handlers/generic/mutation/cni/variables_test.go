// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		VariableName,
		ptr.To(v1alpha1.CNI{}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name:        "unset",
			Vals:        v1alpha1.CNI{},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "set with valid provider",
			Vals: v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCalico,
			},
		},
		capitest.VariableTestDef{
			Name: "set with invalid provider",
			Vals: v1alpha1.CNI{
				Provider: "invalid-provider",
			},
			ExpectError: true,
		},
	)
}
