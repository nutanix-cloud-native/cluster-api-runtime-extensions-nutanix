// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

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
		ptr.To(v1alpha1.OCIImage("").VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "valid",
			Vals: "docker.io/some/image:v2.3.4",
		},
		capitest.VariableTestDef{
			Name:        "invalid",
			Vals:        "this.is.not.valid?",
			ExpectError: true,
		},
	)
}
