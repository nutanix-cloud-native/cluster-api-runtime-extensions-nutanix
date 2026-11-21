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

func usernameClaim(claim string) v1alpha1.PrefixedClaimOrExpression {
	return v1alpha1.PrefixedClaimOrExpression{Claim: claim, Prefix: new("")}
}

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
						Username: usernameClaim("email"),
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
						Username: usernameClaim("email"),
					},
				},
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer-b.example.com",
						Audiences: []string{"aud-b"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: usernameClaim("sub"),
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
						Username: usernameClaim("email"),
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
						Username: usernameClaim("email"),
					},
				},
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"aud-b"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: usernameClaim("sub"),
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
						Username: usernameClaim("email"),
					},
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "claim without prefix",
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
	ExpectError: true,
}, {
	Name: "claim and expression both set",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"e2e-aud"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: v1alpha1.PrefixedClaimOrExpression{
							Claim:      "email",
							Prefix:     new(""),
							Expression: "claims.email",
						},
					},
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "message set on claim validation rule",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"e2e-aud"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: usernameClaim("email"),
					},
					ClaimValidationRules: []v1alpha1.ClaimValidationRule{
						{Claim: "hd", RequiredValue: "example.com", Message: "nope"},
					},
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "multiple audiences without MatchAny",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:       "https://issuer.example.com",
						Audiences: []string{"a", "b"},
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: usernameClaim("email"),
					},
				},
			},
		},
	},
	ExpectError: true,
}, {
	Name: "multiple audiences with MatchAny",
	Vals: v1alpha1.KubeadmClusterConfigSpec{
		Authentication: &v1alpha1.Authentication{
			JWT: []v1alpha1.JWTAuthenticator{
				{
					Issuer: v1alpha1.Issuer{
						URL:                 "https://issuer.example.com",
						Audiences:           []string{"a", "b"},
						AudienceMatchPolicy: v1alpha1.AudienceMatchPolicyMatchAny,
					},
					ClaimMappings: v1alpha1.ClaimMappings{
						Username: usernameClaim("email"),
					},
				},
			},
		},
	},
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
