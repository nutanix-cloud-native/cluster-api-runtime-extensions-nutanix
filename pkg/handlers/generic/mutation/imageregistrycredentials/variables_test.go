// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package imageregistrycredentials

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		variableName,
		ptr.To(v1alpha1.ImageRegistryCredentials{}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "without a Secret",
			Vals: v1alpha1.ImageRegistryCredentials{
				URL: "http://a.b.c.example.com",
			},
		},
		capitest.VariableTestDef{
			Name: "with a Secret",
			Vals: v1alpha1.ImageRegistryCredentials{
				URL: "http://a.b.c.example.com",
				Secret: &corev1.ObjectReference{
					Name: "a.b.c.example.com-creds",
				},
			},
		},
	)
}
