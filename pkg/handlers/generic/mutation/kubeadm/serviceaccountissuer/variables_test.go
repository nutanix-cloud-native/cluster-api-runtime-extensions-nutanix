// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serviceaccountissuer

import (
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

func dockerClusterConfigWithServiceAccountIssuer(
	cfg *v1alpha1.ServiceAccountIssuer,
) v1alpha1.DockerClusterConfigSpec {
	return v1alpha1.DockerClusterConfigSpec{
		KubeadmClusterConfigSpec: v1alpha1.KubeadmClusterConfigSpec{
			ServiceAccountIssuer: cfg,
		},
	}
}

var testDefs = []capitest.VariableTestDef{
	{
		Name: "unset",
		Vals: v1alpha1.DockerClusterConfigSpec{},
	},
	{
		Name: "issuer only",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://oidc.example.com",
		}),
	},
	{
		Name: "issuer with audiences and jwksURI and max expiration",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:             "https://oidc.example.com/id/ABC123",
			Audiences:          []string{"sts.amazonaws.com", "k8s.example.com"},
			JWKSURI:            "https://oidc.example.com/.well-known/jwks.json",
			MaxTokenExpiration: &metav1.Duration{Duration: time.Hour},
		}),
	},
	{
		Name: "issuer with max expiration 24h",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:             "https://oidc.example.com",
			MaxTokenExpiration: &metav1.Duration{Duration: 24 * time.Hour},
		}),
	},
	{
		Name: "issuer empty",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer: "",
		}),
		ExpectError: true,
	},
	{
		Name:        "issuer missing when block present",
		Vals:        dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{}),
		ExpectError: true,
	},
	{
		Name: "issuer http not https",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer: "http://oidc.example.com",
		}),
		ExpectError: true,
	},
	{
		Name: "issuer bare string",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer: "oidc.example.com",
		}),
		ExpectError: true,
	},
	{
		Name: "issuer too long",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer: "https://" + strings.Repeat("a", 2042),
		}),
		ExpectError: true,
	},
	{
		Name: "audiences entry empty",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{""},
		}),
		ExpectError: true,
	},
	{
		Name: "audiences entry too long",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{strings.Repeat("a", 254)},
		}),
		ExpectError: true,
	},
	{
		Name: "audiences duplicate",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:    "https://oidc.example.com",
			Audiences: []string{"a", "a"},
		}),
		ExpectError: true,
	},
	{
		Name: "audiences over max items",
		Vals: func() v1alpha1.DockerClusterConfigSpec {
			auds := make([]string, 51)
			for i := range auds {
				auds[i] = "aud-" + string(rune('a'+i%26)) + strings.Repeat("x", 5)
			}
			return dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
				Issuer:    "https://oidc.example.com",
				Audiences: auds,
			})
		}(),
		ExpectError: true,
	},
	{
		Name: "jwksURI http not https",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:  "https://oidc.example.com",
			JWKSURI: "http://oidc.example.com/jwks",
		}),
		ExpectError: true,
	},
	{
		Name: "jwksURI too long",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:  "https://oidc.example.com",
			JWKSURI: "https://" + strings.Repeat("a", 2042),
		}),
		ExpectError: true,
	},
	{
		Name: "maxTokenExpiration zero",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:             "https://oidc.example.com",
			MaxTokenExpiration: &metav1.Duration{Duration: 0},
		}),
		ExpectError: true,
	},
	{
		Name: "maxTokenExpiration negative",
		Vals: dockerClusterConfigWithServiceAccountIssuer(&v1alpha1.ServiceAccountIssuer{
			Issuer:             "https://oidc.example.com",
			MaxTokenExpiration: &metav1.Duration{Duration: -time.Hour},
		}),
		ExpectError: true,
	},
	{
		Name: "maxTokenExpiration invalid duration string",
		Vals: map[string]any{
			"serviceAccountIssuer": map[string]any{
				"issuer":             "https://oidc.example.com",
				"maxTokenExpiration": "not-a-duration",
			},
		},
		ExpectError: true,
	},
}

func TestVariableValidation_Docker(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		new(v1alpha1.DockerClusterConfig{}.VariableSchema()),
		true,
		dockerclusterconfig.NewVariable,
		testDefs...,
	)
}

func TestVariableValidation_AWS(t *testing.T) {
	awsTestDefs := make([]capitest.VariableTestDef, len(testDefs))
	for i, td := range testDefs {
		awsTestDefs[i] = capitest.VariableTestDef{
			Name:        td.Name,
			ExpectError: td.ExpectError,
		}
		if dockerSpec, ok := td.Vals.(v1alpha1.DockerClusterConfigSpec); ok {
			awsTestDefs[i].Vals = v1alpha1.AWSClusterConfigSpec{
				KubeadmClusterConfigSpec: dockerSpec.KubeadmClusterConfigSpec,
			}
		} else {
			awsTestDefs[i].Vals = td.Vals
		}
	}

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		new(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		awsTestDefs...,
	)
}

func TestVariableValidation_Nutanix(t *testing.T) {
	nutanixTestDefs := make([]capitest.VariableTestDef, len(testDefs))
	for i, td := range testDefs {
		nutanixTestDefs[i] = capitest.VariableTestDef{
			Name:        td.Name,
			ExpectError: td.ExpectError,
		}
		if dockerSpec, ok := td.Vals.(v1alpha1.DockerClusterConfigSpec); ok {
			nutanixTestDefs[i].Vals = v1alpha1.NutanixClusterConfigSpec{
				KubeadmClusterConfigSpec: dockerSpec.KubeadmClusterConfigSpec,
				Nutanix: &v1alpha1.NutanixSpec{
					ControlPlaneEndpoint: v1alpha1.ControlPlaneEndpointSpec{
						Host: "192.168.1.1",
						Port: 9440,
					},
					PrismCentralEndpoint: v1alpha1.NutanixPrismCentralEndpointSpec{
						URL: "https://pc.example.com",
						Credentials: v1alpha1.NutanixPrismCentralEndpointCredentials{
							SecretRef: v1alpha1.LocalObjectReference{Name: "fake-secret"},
						},
					},
				},
				ControlPlane: &v1alpha1.NutanixControlPlaneSpec{
					Nutanix: &v1alpha1.NutanixControlPlaneNodeSpec{
						MachineDetails: v1alpha1.NutanixMachineDetails{
							BootType:       capxv1.NutanixBootTypeLegacy,
							VCPUSockets:    2,
							VCPUsPerSocket: 1,
							Image: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: new("fake-image"),
							},
							Cluster: &capxv1.NutanixResourceIdentifier{
								Type: capxv1.NutanixIdentifierName,
								Name: new("fake-pe-cluster"),
							},
							MemorySize:     resource.MustParse("8Gi"),
							SystemDiskSize: resource.MustParse("40Gi"),
							Subnets: []capxv1.NutanixResourceIdentifier{
								{
									Type: capxv1.NutanixIdentifierName,
									Name: new("fake-subnet"),
								},
							},
						},
					},
				},
			}
		} else {
			nutanixTestDefs[i].Vals = td.Vals
		}
	}

	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		new(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		nutanixTestDefs...,
	)
}
