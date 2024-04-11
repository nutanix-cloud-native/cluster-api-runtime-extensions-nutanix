// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workerconfig

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/workerconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		workerconfig.MetaVariableName,
		ptr.To(v1alpha1.NodeConfigSpec{AWS: &v1alpha1.AWSNodeSpec{}}.VariableSchema()),
		false,
		NewVariable,
		capitest.VariableTestDef{
			Name: "specified IAM instance profile",
			Vals: v1alpha1.NodeConfigSpec{
				AWS: &v1alpha1.AWSNodeSpec{
					IAMInstanceProfile: ptr.To(
						v1alpha1.IAMInstanceProfile("nodes.cluster-api-provider-aws.sigs.k8s.io"),
					),
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified instance type",
			Vals: v1alpha1.NodeConfigSpec{
				AWS: &v1alpha1.AWSNodeSpec{InstanceType: ptr.To(v1alpha1.InstanceType("m5.small"))},
			},
		},
	)
}
