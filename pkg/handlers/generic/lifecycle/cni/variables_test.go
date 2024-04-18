// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.GenericClusterConfig{}.VariableSchema()),
		false,
		clusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "set with valid provider using ClusterResourceSet strategy",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
						Strategy: v1alpha1.AddonStrategyClusterResourceSet,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "set with valid provider using HelmAddon strategy",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Addons: &v1alpha1.Addons{
					CNI: &v1alpha1.CNI{
						Provider: v1alpha1.CNIProviderCalico,
						Strategy: v1alpha1.AddonStrategyHelmAddon,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "set with invalid provider",
			Vals: v1alpha1.GenericClusterConfigSpec{
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
			Vals: v1alpha1.GenericClusterConfigSpec{
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
