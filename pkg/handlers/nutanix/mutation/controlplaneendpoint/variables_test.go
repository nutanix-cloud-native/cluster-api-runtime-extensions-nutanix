// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{Nutanix: &v1alpha1.NutanixSpec{}}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid host or port",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: &v1alpha1.NutanixControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		// capitest.VariableTestDef{
		// 	Name: "empty host or port",
		// 	Vals: v1alpha1.ClusterConfigSpec{
		// 		Nutanix: &v1alpha1.NutanixSpec{
		// 			ControlPlaneEndpoint: &v1alpha1.NutanixControlPlaneEndpointSpec{},
		// 		},
		// 	},
		// },
		// capitest.VariableTestDef{
		// 	Name: "invalid host and port",
		// 	Vals: v1alpha1.ClusterConfigSpec{
		// 		Nutanix: &v1alpha1.NutanixSpec{
		// 			ControlPlaneEndpoint: &v1alpha1.NutanixControlPlaneEndpointSpec{
		// 				Host: "123",
		// 				Port: 0,
		// 			},
		// 		},
		// 	},
		// 	ExpectError: true,
		// },
	)
}
