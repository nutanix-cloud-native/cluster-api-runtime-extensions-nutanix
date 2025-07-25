// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package placementgroup

import (
	"strings"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified placement group name",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							PlacementGroup: &v1alpha1.PlacementGroup{
								Name: "pg-1234",
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified empty group name",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							PlacementGroup: &v1alpha1.PlacementGroup{
								Name: "",
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "specified too long placement group name",
			Vals: v1alpha1.AWSClusterConfigSpec{
				ControlPlane: &v1alpha1.AWSControlPlaneSpec{
					AWS: &v1alpha1.AWSControlPlaneNodeSpec{
						AWSGenericNodeSpec: v1alpha1.AWSGenericNodeSpec{
							PlacementGroup: &v1alpha1.PlacementGroup{
								Name: strings.Repeat("a", 256), // 256 characters long
							},
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}
