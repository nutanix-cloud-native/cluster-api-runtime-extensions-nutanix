// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package noderegistration

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "specified IgnorePreflightErrors",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
					KubeadmNodeSpec: v1alpha1.KubeadmNodeSpec{
						NodeRegistration: &v1alpha1.NodeRegistrationOptions{
							IgnorePreflightErrors: []string{"all"},
						},
					},
				},
			},
		},
	)
}
