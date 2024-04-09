// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package prismcentralendpoint

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	nutanixclusterconfig "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		clusterconfig.MetaVariableName,
		ptr.To(v1alpha1.ClusterConfigSpec{Nutanix: &v1alpha1.NutanixSpec{}}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "valid PC URL",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      fmt.Sprintf("https://prism-central.nutanix.com:%d", v1alpha1.DefaultPrismCentralPort),
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "valid PC URL as an IP",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      fmt.Sprintf("https://10.0.0.1:%d", v1alpha1.DefaultPrismCentralPort),
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "valid PC URL without a port",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      "https://prism-central.nutanix.com",
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "empty PC URL",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "http is not a valid PC URL",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      "http://prism-central.nutanix.com",
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "not a valid PC URL",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      "not-a-valid-url",
						Insecure: false,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "nil PC credentials",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL:      fmt.Sprintf("https://prism-central.nutanix.com:%d", v1alpha1.DefaultPrismCentralPort),
						Insecure: false,
					},
					// ControlPlaneEndpoint is a required field and must always be set
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
				},
			},
			ExpectError: true,
		},
	)
}
