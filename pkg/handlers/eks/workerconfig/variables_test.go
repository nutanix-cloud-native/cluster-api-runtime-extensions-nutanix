// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workerconfig

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.WorkerConfigVariableName,
		ptr.To(v1alpha1.EKSWorkerNodeConfig{}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "specified IAM instance profile",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				EKS: &v1alpha1.AWSWorkerNodeSpec{
					IAMInstanceProfile: "nodes.cluster-api-provider-aws.sigs.k8s.io",
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified instance type",
			Vals: v1alpha1.EKSWorkerNodeConfigSpec{
				EKS: &v1alpha1.AWSWorkerNodeSpec{
					InstanceType: "m5.small",
				},
			},
		},
	)
}
