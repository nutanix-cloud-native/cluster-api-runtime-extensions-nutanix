// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified instance type",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				ControlPlane: &v1alpha1.NutanixNodeConfigSpec{
					GenericNodeSpec: v1alpha1.GenericNodeSpec{
						Taints: []v1alpha1.Taint{{
							Key:    "key",
							Effect: v1alpha1.TaintEffectNoExecute,
							Value:  "value",
						}},
					},
				},
			},
		},
	)
}
