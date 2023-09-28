// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

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
		ptr.To(v1alpha1.ExtraAPIServerCertSANs{}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "single valid SAN",
			Vals: []string{"a.b.c.example.com"},
		},
		capitest.VariableTestDef{
			Name:        "single invalid SAN",
			Vals:        []string{"invalid:san"},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name:        "duplicate valid SANs",
			Vals:        []string{"a.b.c.example.com", "a.b.c.example.com"},
			ExpectError: true,
		},
	)
}
