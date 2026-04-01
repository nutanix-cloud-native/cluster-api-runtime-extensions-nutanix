// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metro

import (
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	emptyMetros := getClusterConfigSpec()

	validMetros := getClusterConfigSpec()
	validMetros.Nutanix.Metros = []string{"metro-1", "metro-2"}

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "empty nutanix metros",
			Vals: emptyMetros,
		},
		capitest.VariableTestDef{
			Name: "valid nonempty nutanix metros",
			Vals: validMetros,
		},
	)
}

func getClusterConfigSpec() *v1alpha1.NutanixClusterConfigSpec {
	return &v1alpha1.NutanixClusterConfigSpec{
		Nutanix: &v1alpha1.NutanixSpec{
			ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
				Host: "10.20.100.10",
				Port: 6443,
			},
			// PrismCentralEndpoint is a required field and must always be set
			PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
				URL: "https://prism-central.nutanix.com:9440",
				Credentials: v1alpha1.NutanixPrismCentralEndpointCredentials{
					SecretRef: v1alpha1.LocalObjectReference{
						Name: "credentials",
					},
				},
			},
			Metros: []string{},
		},
	}
}
