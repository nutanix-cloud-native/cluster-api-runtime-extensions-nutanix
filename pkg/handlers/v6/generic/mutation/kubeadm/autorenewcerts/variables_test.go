// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package autorenewcerts

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var nutanixTestDefs = []capitest.VariableTestDef{
	{
		Name: "unset",
		Vals: v1alpha1.NutanixClusterConfigSpec{
			ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
				GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{},
			},
		},
	},
	{
		Name: "set with a valid value of 0",
		Vals: v1alpha1.NutanixClusterConfigSpec{
			ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
				GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{
					AutoRenewCertificates: &v1alpha1.AutoRenewCertificatesSpec{
						DaysBeforeExpiry: 0,
					},
				},
			},
		},
	},
	{
		Name: "set with a minimum valid value of 7",
		Vals: v1alpha1.NutanixClusterConfigSpec{
			ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
				GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{
					AutoRenewCertificates: &v1alpha1.AutoRenewCertificatesSpec{
						DaysBeforeExpiry: 7,
					},
				},
			},
		},
	},
	{
		Name: "set with an invalid value",
		Vals: v1alpha1.NutanixClusterConfigSpec{
			ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
				GenericControlPlaneSpec: v1alpha1.GenericControlPlaneSpec{
					AutoRenewCertificates: &v1alpha1.AutoRenewCertificatesSpec{
						DaysBeforeExpiry: 1,
					},
				},
			},
		},
		ExpectError: true,
	},
}

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariablesAs[mutation.DiscoverVariables, v1alpha1.NutanixClusterConfigSpec](
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		func() mutation.DiscoverVariables {
			return nutanixclusterconfig.NewVariable()
		},
		nutanixTestDefs...,
	)
}
