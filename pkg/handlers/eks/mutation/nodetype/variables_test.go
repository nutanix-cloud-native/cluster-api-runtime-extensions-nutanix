// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nodetype

import (
	"testing"

	"k8s.io/utils/ptr"

	eksbootstrapv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/bootstrap/eks/api/v1beta2"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	eksworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/workerconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.EKSWorkerNodeConfig{}.VariableSchema()),
		false,
		eksworkerconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified valid node type",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				EKS: &v1alpha1.EKSWorkerNodeSpec{
					NodeType: string(eksbootstrapv1.NodeTypeAL2023),
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified invalid node type",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				EKS: &v1alpha1.EKSWorkerNodeSpec{
					NodeType: "invalid",
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "empty node type",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				EKS: &v1alpha1.EKSWorkerNodeSpec{
					NodeType: "",
				},
			},
		},
	)
}
