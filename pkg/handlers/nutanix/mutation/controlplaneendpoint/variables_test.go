// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
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
			Name: "valid host and port",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 6443,
					},
					// PrismCentralEndpoint is a required field and must always be set
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						Host: "prism-central.nutanix.com",
						Port: v1alpha1.PrismCentralPort,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "empty host",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "",
						Port: 6443,
					},
					// PrismCentralEndpoint is a required field and must always be set
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						Host: "prism-central.nutanix.com",
						Port: v1alpha1.PrismCentralPort,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
				},
			},
			ExpectError: true,
		},
		capitest.VariableTestDef{
			Name: "port set to 0",
			Vals: v1alpha1.ClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: clusterv1.APIEndpoint{
						Host: "10.20.100.10",
						Port: 0,
					},
					// PrismCentralEndpoint is a required field and must always be set
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						Host: "prism-central.nutanix.com",
						Port: v1alpha1.PrismCentralPort,
						Credentials: corev1.LocalObjectReference{
							Name: "credentials",
						},
					},
				},
			},
			ExpectError: true,
		},
	)
}
