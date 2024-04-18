// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

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
			Name: "ClusterResourceSet strategy",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Addons: &v1alpha1.Addons{
					ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
						Strategy: v1alpha1.AddonStrategyClusterResourceSet,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "HelmAddon strategy",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Addons: &v1alpha1.Addons{
					ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
						Strategy: v1alpha1.AddonStrategyHelmAddon,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid strategy",
			Vals: v1alpha1.GenericClusterConfigSpec{
				Addons: &v1alpha1.Addons{
					ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
						Strategy: "invalid-strategy",
					},
				},
			},
			ExpectError: true,
		},
	)
}
