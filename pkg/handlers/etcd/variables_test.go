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
			Name: "set",
			Vals: v1alpha1.Etcd{
				ImageRepository: "my-registry.io/my-org/my-repo",
				ImageTag:        "v3.5.99_custom.0",
			},
		},
	)
}
