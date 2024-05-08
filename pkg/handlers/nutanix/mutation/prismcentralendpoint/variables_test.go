// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package prismcentralendpoint

import (
	"fmt"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid PC URL",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: fmt.Sprintf(
							"https://prism-central.nutanix.com:%d",
							v1alpha1.DefaultPrismCentralPort,
						),
						Insecure: false,
						Credentials: v1alpha1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "valid PC URL as an IP",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: fmt.Sprintf(
							"https://10.0.0.1:%d",
							v1alpha1.DefaultPrismCentralPort,
						),
						Insecure: false,
						Credentials: v1alpha1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "valid PC URL without a port",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      "https://prism-central.nutanix.com",
						Insecure: false,
						Credentials: v1alpha1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "empty PC URL",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						Insecure: false,
						Credentials: v1alpha1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "http is not a valid PC URL",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      "http://prism-central.nutanix.com",
						Insecure: false,
						Credentials: v1alpha1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "not a valid PC URL",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      "not-a-valid-url",
						Insecure: false,
						Credentials: v1alpha1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "nil PC credentials",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: fmt.Sprintf(
							"https://prism-central.nutanix.com:%d",
							v1alpha1.DefaultPrismCentralPort,
						),
						Insecure: false,
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
	)
}
