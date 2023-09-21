// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		variableName,
		ptr.To(v1alpha1.HTTPProxy{}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "valid values",
			Vals: v1alpha1.HTTPProxy{
				HTTP:         "http://a.b.c.example.com",
				HTTPS:        "https://a.b.c.example.com",
				AdditionalNo: []string{"d.e.f.example.com"},
			},
		},
	)
}
