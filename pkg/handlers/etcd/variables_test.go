// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package etcd

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
		ptr.To(v1alpha1.Etcd{}.VariableSchema()),
		NewVariable,
		capitest.VariableTestDef{
			Name: "unset",
			Vals: v1alpha1.Etcd{},
		},
		capitest.VariableTestDef{
			Name: "set with valid image values",
			Vals: v1alpha1.Etcd{
				Image: &v1alpha1.Image{
					Repository: "my-registry.io/my-org/my-repo",
					Tag:        "v3.5.99_custom.0",
				},
			},
		},
		capitest.VariableTestDef{
			Name: "set with invalid image repository",
			Vals: v1alpha1.Etcd{
				Image: &v1alpha1.Image{
					Repository: "https://this.should.not.have.a.scheme",
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "set with invalid image tag",
			Vals: v1alpha1.Etcd{
				Image: &v1alpha1.Image{
					Tag: "this:is:not:a:valid:tag",
				},
			},
			ExpectError: true,
		},
	)
}
