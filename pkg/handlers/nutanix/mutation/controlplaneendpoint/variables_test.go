// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"fmt"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testPrismCentralURL = fmt.Sprintf(
	"https://prism-central.nutanix.com:%d",
	v1alpha1.DefaultPrismCentralPort,
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid host and port",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
					// PrismCentralEndpoint is a required field and must always be set
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: testPrismCentralURL,
						Credentials: v1alpha1.NutanixPrismCentralEndpointCredentials{
							SecretRef: v1alpha1.LocalObjectReference{
								Name: "credentials",
							},
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "empty host",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "",
						Port: 6443,
					},
					// PrismCentralEndpoint is a required field and must always be set
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: testPrismCentralURL,
						Credentials: v1alpha1.NutanixPrismCentralEndpointCredentials{
							SecretRef: v1alpha1.LocalObjectReference{
								Name: "credentials",
							},
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "port set to 0",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 0,
					},
					// PrismCentralEndpoint is a required field and must always be set
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: testPrismCentralURL,
						Credentials: v1alpha1.NutanixPrismCentralEndpointCredentials{
							SecretRef: v1alpha1.LocalObjectReference{
								Name: "credentials",
							},
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}
