// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

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

var testDefs = []capitest.VariableTestDef{{
	Name: "set with valid provider using ClusterResourceSet strategy",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CNI: &v1alpha1.CNI{
					Provider: v1alpha1.CNIProviderCalico,
					Strategy: v1alpha1.AddonStrategyClusterResourceSet,
				},
			},
		},
	},
}, {
	Name: "set with valid provider using HelmAddon strategy",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CNI: &v1alpha1.CNI{
					Provider: v1alpha1.CNIProviderCalico,
					Strategy: v1alpha1.AddonStrategyHelmAddon,
				},
			},
		},
	},
}, {
	Name: "set with valid provider using HelmAddon strategy and custom helm values",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CNI: &v1alpha1.CNI{
					Provider: v1alpha1.CNIProviderCilium,
					Strategy: v1alpha1.AddonStrategyHelmAddon,
					AddonConfig: v1alpha1.AddonConfig{
						Values: &v1alpha1.AddonValues{
							SourceRef: &v1alpha1.ValuesReference{
								Name: "custom-cilium-cni-helm-values",
								Kind: "ConfigMap",
							},
						},
					},
				},
			},
		},
	},
}, {
	Name: "set with invalid provider",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CNI: &v1alpha1.CNI{
					Provider: "invalid-provider",
					Strategy: v1alpha1.AddonStrategyClusterResourceSet,
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "set with invalid strategy",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			GenericAddons: v1alpha1.GenericAddons{
				CNI: &v1alpha1.CNI{
					Provider: v1alpha1.CNIProviderCalico,
					Strategy: v1alpha1.AddonStrategy("invalid-strategy"),
				},
			},
		},
	},
	ExpectError: true,
}}

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		testDefs...,
	)
}
