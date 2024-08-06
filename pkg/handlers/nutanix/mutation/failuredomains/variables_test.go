// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomains

import (
	"fmt"
	"testing"

	"k8s.io/utils/ptr"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testPrismCentralURL = fmt.Sprintf(
	"https://prism-central.nutanix.com:%d",
	v1alpha1.DefaultPrismCentralPort,
)

var (
	testPEName     string = "PE1"
	testSubnetName string = "Subnet1"
)

func TestVariableValidation(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		ptr.To(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		capitest.VariableTestDef{
			Name: "failure domains not provided",
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
			Name: "1 failure domain provided",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					// ControlPlaneEndpoint is a required field and must always be set
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
					FailureDomains: v1alpha1.NutanixFailureDomains{
						v1alpha1.NutanixFailureDomain{
							Name: "fd1",
							Cluster: v1beta1.NutanixResourceIdentifier{
								Type: v1beta1.NutanixIdentifierName,
								Name: &testPEName,
							},
							Subnets: []v1beta1.NutanixResourceIdentifier{
								{
									Type: v1beta1.NutanixIdentifierName,
									Name: &testSubnetName,
								},
							},
							ControlPlane: true,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "multiple failure domains provided",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					// ControlPlaneEndpoint is a required field and must always be set
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
					FailureDomains: v1alpha1.NutanixFailureDomains{
						v1alpha1.NutanixFailureDomain{
							Name: "fd1",
							Cluster: v1beta1.NutanixResourceIdentifier{
								Type: v1beta1.NutanixIdentifierName,
								Name: &testPEName,
							},
							Subnets: []v1beta1.NutanixResourceIdentifier{
								{
									Type: v1beta1.NutanixIdentifierName,
									Name: &testSubnetName,
								},
							},
							ControlPlane: true,
						},
						v1alpha1.NutanixFailureDomain{
							Name: "fd2",
							Cluster: v1beta1.NutanixResourceIdentifier{
								Type: v1beta1.NutanixIdentifierName,
								Name: &testPEName,
							},
							Subnets: []v1beta1.NutanixResourceIdentifier{
								{
									Type: v1beta1.NutanixIdentifierName,
									Name: &testSubnetName,
								},
							},
							ControlPlane: true,
						},
					},
				},
			},
		},
		capitest.VariableTestDef{
			Name: "duplicate named failure domains provided",
			Vals: v1alpha1.NutanixClusterConfigSpec{
				Nutanix: &v1alpha1.NutanixSpec{
					// ControlPlaneEndpoint is a required field and must always be set
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
					FailureDomains: v1alpha1.NutanixFailureDomains{
						v1alpha1.NutanixFailureDomain{
							Name: "fd1",
							Cluster: v1beta1.NutanixResourceIdentifier{
								Type: v1beta1.NutanixIdentifierName,
								Name: &testPEName,
							},
							Subnets: []v1beta1.NutanixResourceIdentifier{
								{
									Type: v1beta1.NutanixIdentifierName,
									Name: &testSubnetName,
								},
							},
							ControlPlane: true,
						},
						v1alpha1.NutanixFailureDomain{
							Name: "fd1",
							Cluster: v1beta1.NutanixResourceIdentifier{
								Type: v1beta1.NutanixIdentifierName,
								Name: &testPEName,
							},
							Subnets: []v1beta1.NutanixResourceIdentifier{
								{
									Type: v1beta1.NutanixIdentifierName,
									Name: &testSubnetName,
								},
							},
							ControlPlane: true,
						},
					},
				},
			},
		},
	)
}
