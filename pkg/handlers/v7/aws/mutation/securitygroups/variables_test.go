// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package securitygroups

import (
	"testing"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		new(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "Additional Security Group Specification",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							AdditionalSecurityGroups: v1alpha1.AdditionalSecurityGroup{
								{
									ID: "sg-1234",
								},
								{
									ID: "sg-0420",
								},
							},
						},
					},
				},
			},
		},
	)
}
