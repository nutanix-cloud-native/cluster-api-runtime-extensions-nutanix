// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"testing"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
)

var testDefs = []capitest.VariableTestDef{{
	Name: "unset",
	Vals: v1alpha1.KubeadmClusterConfigSpec{},
}, {
	Name: "set with valid single issuer",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"e2e-aud"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
					},
				},
			},
		},
	},
}, {
	Name: "set with valid two issuers",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer-a.example.com",
						Audiences: []string{"aud-a"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
					},
				},
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer-b.example.com",
						Audiences: []string{"aud-b"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"},
					},
				},
			},
		},
	},
}, {
	Name: "set with invalid issuer URL not https",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "http://issuer.example.com",
						Audiences: []string{"e2e-aud"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
					},
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "set with duplicate issuer URLs",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"aud-a"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
					},
				},
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"aud-b"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"},
					},
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "set with missing audiences",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
					},
				},
			},
		},
	},
	ExpectError: true,
}}

func TestVariableValidation_AWS(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		new(v1alpha1.AWSClusterConfig{}.VariableSchema()),
		true,
		awsclusterconfig.NewVariable,
		testDefs...,
	)
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

func TestVariableValidation_Nutanix(t *testing.T) {
	capitest.ValidateDiscoverVariables(
		t,
		v1alpha1.ClusterConfigVariableName,
		new(v1alpha1.NutanixClusterConfig{}.VariableSchema()),
		true,
		nutanixclusterconfig.NewVariable,
		testDefs...,
	)
}
