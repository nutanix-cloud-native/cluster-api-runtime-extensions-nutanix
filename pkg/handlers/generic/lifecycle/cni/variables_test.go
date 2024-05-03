// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{{
	Name: "set with valid provider using ClusterResourceSet strategy",
	Vals: v1alpha1.GenericClusterConfigSpec{
		Addons: &v1alpha1.Addons{
			CNI: &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCalico,
				Strategy: v1alpha1.AddonStrategyClusterResourceSet,
			},
		},
	},
}, {
	Name: "set with valid provider using HelmAddon strategy",
	Vals: v1alpha1.GenericClusterConfigSpec{
		Addons: &v1alpha1.Addons{
			CNI: &v1alpha1.CNI{
				Provider: v1alpha1.CNIProviderCalico,
				Strategy: v1alpha1.AddonStrategyHelmAddon,
			},
		},
	},
}, {
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
}, {
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
