// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.GenericClusterConfig{}.VariableSchema()),
		false,
		clusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "set with valid provider",
			Vals: v1alpha1.GenericClusterConfig{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
						Strategy: v1alpha1.AddonStrategyClusterResourceSet,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "set with invalid provider",
			Vals: v1alpha1.GenericClusterConfig{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: "invalid-provider",
						Strategy: v1alpha1.AddonStrategyClusterResourceSet,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "set with invalid strategy",
			Vals: v1alpha1.GenericClusterConfig{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
						Strategy: "invalid-strategy",
					},
				},
			},
			ExpectError: true,
		},
	)
}
