// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package k8sregistrationagent

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{{
	Name: "HelmAddon strategy",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			k8sRegistrationAgent: &apivariables.NutanixK8sRegistrationAgent{
				Strategy: v1alpha1.AddonStrategyHelmAddon,
			},
		},
	},
}, {
	Name: "ClusterResourceSet strategy",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			k8sRegistrationAgent: &apivariables.NutanixK8sRegistrationAgent{
				Strategy: v1alpha1.AddonStrategyClusterResourceSet,
			},
		},
	},
	ExpectError: true,
}, {
	Name: "invalid strategy",
	Vals: apivariables.ClusterConfigSpec{
		Addons: &apivariables.Addons{
			k8sRegistrationAgent: &apivariables.NutanixK8sRegistrationAgent{
				Strategy: v1alpha1.AddonStrategy("invalid-strategy"),
			},
		},
	},
	ExpectError: true,
}}

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
