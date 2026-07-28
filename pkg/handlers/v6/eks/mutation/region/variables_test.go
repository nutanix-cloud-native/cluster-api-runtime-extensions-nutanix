// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package region

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
			Name: "specified region",
			Vals: v1alpha1.EKSClusterConfigSpec{
				EKS: &v1alpha1.EKSSpec{
					Region: ptr.To(v1alpha1.Region("specified-region")),
				},
			},
		},
	)
}
