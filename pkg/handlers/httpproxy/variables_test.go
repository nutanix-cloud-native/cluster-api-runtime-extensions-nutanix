// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		VariableName,
		ptr.To(HTTPProxyVariables{}.VariableSchema()),
		NewVariable,
		capitest.VariableTestDef{
			Name: "valid values",
			Vals: HTTPProxyVariables{
				HTTP:         "http://a.b.c.example.com",
				HTTPS:        "https://a.b.c.example.com",
				AdditionalNo: []string{"d.e.f.example.com"},
			},
		},
	)
}
