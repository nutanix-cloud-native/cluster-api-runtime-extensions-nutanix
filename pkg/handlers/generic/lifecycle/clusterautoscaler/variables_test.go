// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterautoscaler

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "ClusterResourceSet strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategyClusterResourceSet,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "HelmAddon strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategyHelmAddon,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategy("invalid-strategy"),
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "ClusterResourceSet strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategyClusterResourceSet,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "HelmAddon strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategyHelmAddon,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategy("invalid-strategy"),
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "ClusterResourceSet strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategyClusterResourceSet,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "HelmAddon strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategyHelmAddon,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "invalid strategy",
			Vals: apivariables.ClusterConfigSpec{
				Addons: &apivariables.Addons{
					GenericAddons: v1alpha1.GenericAddons{
						ClusterAutoscaler: &v1alpha1.ClusterAutoscaler{
							Strategy: v1alpha1.AddonStrategy("invalid-strategy"),
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}
