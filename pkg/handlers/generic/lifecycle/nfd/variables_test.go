// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nfd

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.AllProvidersSpec{}.VariableSchema()),
		false,
		clusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "ClusterResourceSet strategy",
			Vals: v1alpha1.AllProvidersSpec{
				Addons: &v1alpha1.Addons{
					NFD: &v1alpha1.NFD{
						Strategy: v1alpha1.AddonStrategyClusterResourceSet,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "HelmAddon strategy",
			Vals: v1alpha1.AllProvidersSpec{
				Addons: &v1alpha1.Addons{
					NFD: &v1alpha1.NFD{
						Strategy: v1alpha1.AddonStrategyHelmAddon,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid strategy",
			Vals: v1alpha1.AllProvidersSpec{
				Addons: &v1alpha1.Addons{
					NFD: &v1alpha1.NFD{
						Strategy: "invalid-strategy",
					},
				},
			},
			ExpectError: true,
		},
	)
}
