// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func strPtr(s string) *string { return &s }

func mustRender(t *testing.T, auth *v1alpha1.Authentication) (apiserverv1.AuthenticationConfiguration, string) {
	t.Helper()
	y, err := renderAuthenticationConfig(auth)
	require.NoError(t, err)
	var cfg apiserverv1.AuthenticationConfiguration
	require.NoError(t, yaml.Unmarshal([]byte(y), &cfg))
	return cfg, y
}

func TestRenderAuthenticationConfig(t *testing.T) {
	tests := []struct {
		name   string
		input  *v1alpha1.Authentication
		verify func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, raw string)
	}{
		{
			name: "apiVersion and kind are set",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, raw string) {
				t.Helper()
				assert.Equal(t, "apiserver.config.k8s.io/v1", cfg.APIVersion)
				assert.Equal(t, "AuthenticationConfiguration", cfg.Kind)
				assert.Contains(t, raw, "apiserver.config.k8s.io/v1")
				assert.Contains(t, raw, "AuthenticationConfiguration")
			},
		},
		{
			name: "single issuer minimal",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT, 1)
				assert.Equal(t, "https://issuer.example.com", cfg.JWT[0].Issuer.URL)
				assert.Equal(t, []string{"k8s"}, cfg.JWT[0].Issuer.Audiences)
				assert.Nil(t, cfg.JWT[0].Issuer.DiscoveryURL)
				assert.Empty(t, cfg.JWT[0].Issuer.CertificateAuthority)
				assert.Empty(t, string(cfg.JWT[0].Issuer.AudienceMatchPolicy))
				assert.Equal(t, "email", cfg.JWT[0].ClaimMappings.Username.Claim)
				assert.Nil(t, cfg.JWT[0].ClaimMappings.Username.Prefix)
				assert.Empty(t, cfg.JWT[0].ClaimMappings.Username.Expression)
			},
		},
		{
			name: "discoveryURL set",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{
							URL:          "https://issuer.example.com",
							DiscoveryURL: "https://discovery.example.com/.well-known/openid-configuration",
							Audiences:    []string{"k8s"},
						},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.NotNil(t, cfg.JWT[0].Issuer.DiscoveryURL)
				assert.Equal(t, "https://discovery.example.com/.well-known/openid-configuration", *cfg.JWT[0].Issuer.DiscoveryURL)
			},
		},
		{
			name: "discoveryURL empty omitted",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", DiscoveryURL: "", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Nil(t, cfg.JWT[0].Issuer.DiscoveryURL, "empty discoveryURL should be nil via stringPtrOrNil")
			},
		},
		{
			name: "certificateAuthority",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{
							URL:                  "https://issuer.example.com",
							Audiences:            []string{"k8s"},
							CertificateAuthority: "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----",
						},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----", cfg.JWT[0].Issuer.CertificateAuthority)
			},
		},
		{
			name: "audienceMatchPolicy MatchAny",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{
							URL:                 "https://issuer.example.com",
							Audiences:           []string{"a", "b"},
							AudienceMatchPolicy: v1alpha1.AudienceMatchPolicyMatchAny,
						},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, apiserverv1.AudienceMatchPolicyType("MatchAny"), cfg.JWT[0].Issuer.AudienceMatchPolicy)
			},
		},
		{
			name: "audienceMatchPolicy empty omitted",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Empty(t, string(cfg.JWT[0].Issuer.AudienceMatchPolicy))
			},
		},
		{
			name: "audiences order preserved",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"c", "a", "b"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, []string{"c", "a", "b"}, cfg.JWT[0].Issuer.Audiences)
			},
		},
		{
			name: "claimValidationRules claim+requiredValue",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
						ClaimValidationRules: []v1alpha1.ClaimValidationRule{
							{Claim: "hd", RequiredValue: "example.com"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT[0].ClaimValidationRules, 1)
				assert.Equal(t, "hd", cfg.JWT[0].ClaimValidationRules[0].Claim)
				assert.Equal(t, "example.com", cfg.JWT[0].ClaimValidationRules[0].RequiredValue)
				assert.Empty(t, cfg.JWT[0].ClaimValidationRules[0].Expression)
			},
		},
		{
			name: "claimValidationRules expression with message",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
						ClaimValidationRules: []v1alpha1.ClaimValidationRule{
							{Expression: "claims.exp - claims.nbf <= 86400", Message: "token lifetime too long"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT[0].ClaimValidationRules, 1)
				assert.Equal(t, "claims.exp - claims.nbf <= 86400", cfg.JWT[0].ClaimValidationRules[0].Expression)
				assert.Equal(t, "token lifetime too long", cfg.JWT[0].ClaimValidationRules[0].Message)
				assert.Empty(t, cfg.JWT[0].ClaimValidationRules[0].Claim)
			},
		},
		{
			name: "claimValidationRules preserve order",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
						ClaimValidationRules: []v1alpha1.ClaimValidationRule{
							{Claim: "hd", RequiredValue: "example.com"},
							{Expression: "claims.exp - claims.nbf <= 86400"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT[0].ClaimValidationRules, 2)
				assert.Equal(t, "hd", cfg.JWT[0].ClaimValidationRules[0].Claim)
				assert.Equal(t, "claims.exp - claims.nbf <= 86400", cfg.JWT[0].ClaimValidationRules[1].Expression)
			},
		},
		{
			name: "userValidationRules",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
						UserValidationRules: []v1alpha1.UserValidationRule{
							{Expression: "!user.username.startsWith('system:')", Message: "system: prefix reserved"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT[0].UserValidationRules, 1)
				assert.Equal(t, "!user.username.startsWith('system:')", cfg.JWT[0].UserValidationRules[0].Expression)
				assert.Equal(t, "system: prefix reserved", cfg.JWT[0].UserValidationRules[0].Message)
			},
		},
		{
			name: "username claim with custom prefix",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email", Prefix: strPtr("myco:")},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, "email", cfg.JWT[0].ClaimMappings.Username.Claim)
				require.NotNil(t, cfg.JWT[0].ClaimMappings.Username.Prefix)
				assert.Equal(t, "myco:", *cfg.JWT[0].ClaimMappings.Username.Prefix)
			},
		},
		{
			name: "username prefix dash disables prefix",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email", Prefix: strPtr("-")},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.NotNil(t, cfg.JWT[0].ClaimMappings.Username.Prefix)
				assert.Equal(t, "-", *cfg.JWT[0].ClaimMappings.Username.Prefix)
			},
		},
		{
			name: "username expression",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Expression: `claims.email + ":ext"`},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, `claims.email + ":ext"`, cfg.JWT[0].ClaimMappings.Username.Expression)
				assert.Empty(t, cfg.JWT[0].ClaimMappings.Username.Claim)
			},
		},
		{
			name: "groups claim with prefix",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
							Groups:   &v1alpha1.PrefixedClaimOrExpression{Claim: "groups", Prefix: strPtr("")},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, "groups", cfg.JWT[0].ClaimMappings.Groups.Claim)
				require.NotNil(t, cfg.JWT[0].ClaimMappings.Groups.Prefix)
				assert.Equal(t, "", *cfg.JWT[0].ClaimMappings.Groups.Prefix)
			},
		},
		{
			name: "groups expression",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
							Groups:   &v1alpha1.PrefixedClaimOrExpression{Expression: `claims.groups.split(",")`},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, `claims.groups.split(",")`, cfg.JWT[0].ClaimMappings.Groups.Expression)
			},
		},
		{
			name: "uid claim",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
							UID:      &v1alpha1.ClaimOrExpression{Claim: "sub"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, "sub", cfg.JWT[0].ClaimMappings.UID.Claim)
			},
		},
		{
			name: "uid expression",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
							UID:      &v1alpha1.ClaimOrExpression{Expression: `claims.sub`},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				assert.Equal(t, "claims.sub", cfg.JWT[0].ClaimMappings.UID.Expression)
			},
		},
		{
			name: "extra single and multiple preserve order",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{URL: "https://issuer.example.com", Audiences: []string{"k8s"}},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"},
							Extra: []v1alpha1.ExtraMapping{
								{Key: "example.org/foo", ValueExpression: "'bar'"},
								{Key: "example.org/admin", ValueExpression: `claims.is_admin ? "true" : ""`},
							},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT[0].ClaimMappings.Extra, 2)
				assert.Equal(t, "example.org/foo", cfg.JWT[0].ClaimMappings.Extra[0].Key)
				assert.Equal(t, "'bar'", cfg.JWT[0].ClaimMappings.Extra[0].ValueExpression)
				assert.Equal(t, "example.org/admin", cfg.JWT[0].ClaimMappings.Extra[1].Key)
			},
		},
		{
			name: "full authenticator all fields",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer: v1alpha1.Issuer{
							URL:                  "https://issuer.example.com",
							DiscoveryURL:         "https://discovery.example.com/.well-known/openid-configuration",
							CertificateAuthority: "pem-bundle",
							Audiences:            []string{"aud1", "aud2"},
							AudienceMatchPolicy:  v1alpha1.AudienceMatchPolicyMatchAny,
						},
						ClaimValidationRules: []v1alpha1.ClaimValidationRule{
							{Claim: "hd", RequiredValue: "example.com"},
							{Expression: "claims.exp - claims.nbf <= 86400", Message: "lifetime"},
						},
						ClaimMappings: v1alpha1.ClaimMappings{
							Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email", Prefix: strPtr("co:")},
							Groups:   &v1alpha1.PrefixedClaimOrExpression{Claim: "groups", Prefix: strPtr("")},
							UID:      &v1alpha1.ClaimOrExpression{Expression: "claims.sub"},
							Extra: []v1alpha1.ExtraMapping{
								{Key: "example.org/foo", ValueExpression: "claims.foo"},
							},
						},
						UserValidationRules: []v1alpha1.UserValidationRule{
							{Expression: "!user.username.startsWith('system:')"},
						},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT, 1)
				j := cfg.JWT[0]
				assert.Equal(t, "https://issuer.example.com", j.Issuer.URL)
				require.NotNil(t, j.Issuer.DiscoveryURL)
				assert.Equal(t, "https://discovery.example.com/.well-known/openid-configuration", *j.Issuer.DiscoveryURL)
				assert.Equal(t, "pem-bundle", j.Issuer.CertificateAuthority)
				assert.Equal(t, []string{"aud1", "aud2"}, j.Issuer.Audiences)
				assert.Equal(t, apiserverv1.AudienceMatchPolicyType("MatchAny"), j.Issuer.AudienceMatchPolicy)
				require.Len(t, j.ClaimValidationRules, 2)
				assert.Equal(t, "hd", j.ClaimValidationRules[0].Claim)
				assert.Equal(t, "claims.exp - claims.nbf <= 86400", j.ClaimValidationRules[1].Expression)
				assert.Equal(t, "co:", *j.ClaimMappings.Username.Prefix)
				assert.Equal(t, "groups", j.ClaimMappings.Groups.Claim)
				assert.Equal(t, "claims.sub", j.ClaimMappings.UID.Expression)
				require.Len(t, j.ClaimMappings.Extra, 1)
				assert.Equal(t, "example.org/foo", j.ClaimMappings.Extra[0].Key)
				require.Len(t, j.UserValidationRules, 1)
			},
		},
		{
			name: "two issuers preserve order",
			input: &v1alpha1.Authentication{
				JWT: []v1alpha1.JWTAuthenticator{
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer-a.example.com", Audiences: []string{"aud-a"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "email"}},
					},
					{
						Issuer:        v1alpha1.Issuer{URL: "https://issuer-b.example.com", Audiences: []string{"aud-b-1", "aud-b-2"}},
						ClaimMappings: v1alpha1.ClaimMappings{Username: v1alpha1.PrefixedClaimOrExpression{Claim: "sub"}},
					},
				},
			},
			verify: func(t *testing.T, cfg apiserverv1.AuthenticationConfiguration, _ string) {
				t.Helper()
				require.Len(t, cfg.JWT, 2)
				assert.Equal(t, "https://issuer-a.example.com", cfg.JWT[0].Issuer.URL)
				assert.Equal(t, "https://issuer-b.example.com", cfg.JWT[1].Issuer.URL)
				assert.Equal(t, []string{"aud-a"}, cfg.JWT[0].Issuer.Audiences)
				assert.Equal(t, []string{"aud-b-1", "aud-b-2"}, cfg.JWT[1].Issuer.Audiences)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, raw := mustRender(t, tt.input)
			tt.verify(t, cfg, raw)
		})
	}
}

func TestStringPtrOrNil(t *testing.T) {
	assert.Nil(t, stringPtrOrNil(""))
	assert.Equal(t, "x", *stringPtrOrNil("x"))
}
