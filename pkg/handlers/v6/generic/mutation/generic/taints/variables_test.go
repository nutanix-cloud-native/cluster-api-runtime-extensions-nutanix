// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	eksworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/workerconfig"
	nutanixworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/workerconfig"
)

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.NutanixWorkerNodeConfig{}.VariableSchema()),
		false,
		nutanixworkerconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified nodepool taints",
			Vals: v1alpha1.NutanixWorkerNodeConfigSpec{
				GenericNodeSpec: v1alpha1.GenericNodeSpec{
					Taints: []v1alpha1.Taint{{
						Key:    "key",
						Effect: v1alpha1.TaintEffectNoExecute,
						Value:  "value",
					}},
				},
			},
		},
	)
}

func TestVariableValidation_EKS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.EKSWorkerNodeConfig{}.VariableSchema()),
		false,
		eksworkerconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified nodepool taints",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				GenericNodeSpec: v1alpha1.GenericNodeSpec{
					Taints: []v1alpha1.Taint{{
						Key:    "key",
						Effect: v1alpha1.TaintEffectNoExecute,
						Value:  "value",
					}},
				},
			},
		},
	)
}
