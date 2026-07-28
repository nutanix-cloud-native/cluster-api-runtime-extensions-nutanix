// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package network

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	eksclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.EKSClusterConfig{}.VariableSchema()),
		true,
		eksclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified VPC ID",
			Vals: v1alpha1.EKSClusterConfigSpec{
				EKS: &v1alpha1.EKSSpec{
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
			Vals: v1alpha1.EKSClusterConfigSpec{
				EKS: &v1alpha1.EKSSpec{
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
			Vals: v1alpha1.EKSClusterConfigSpec{
				EKS: &v1alpha1.EKSSpec{
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
