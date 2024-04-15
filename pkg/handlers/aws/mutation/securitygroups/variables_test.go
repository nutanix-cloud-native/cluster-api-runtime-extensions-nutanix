// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package securitygroups

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.NewAWSClusterConfigSpec().VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "Additional Security Group Specification",
			Vals: v1alpha1.ClusterConfigSpec{
				ControlPlane: &v1alpha1.NodeConfigSpec{
					AWS: &v1alpha1.AWSNodeSpec{
						AdditionalSecurityGroups: v1alpha1.AdditionalSecurityGroup{
							{
								ID: ptr.To("sg-1234"),
							},
							{
								ID: ptr.To("sg-0420"),
							},
						},
					},
				},
			},
		},
	)
}
