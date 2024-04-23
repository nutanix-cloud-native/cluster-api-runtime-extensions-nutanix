// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package network

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
		ptr.To(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified VPC ID",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					Network: &v1alpha1.AWSNetwork{
						VPC: &v1alpha1.VPC{
							ID: "vpc-1234",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified subnet IDs",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					Network: &v1alpha1.AWSNetwork{
						Subnets: v1alpha1.Subnets{
							{ID: "subnet-1"},
							{ID: "subnet-2"},
							{ID: "subnet-3"},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "specified both VPC ID and subnet IDs",
			Vals: v1alpha1.AWSClusterConfigSpec{
				AWS: &v1alpha1.AWSSpec{
					Network: &v1alpha1.AWSNetwork{
						VPC: &v1alpha1.VPC{
							ID: "vpc-1234",
						},
						Subnets: v1alpha1.Subnets{
							{ID: "subnet-1"},
							{ID: "subnet-2"},
							{ID: "subnet-3"},
						},
					},
				},
			},
		},
	)
}
