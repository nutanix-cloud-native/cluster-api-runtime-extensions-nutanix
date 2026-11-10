// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceAccountIssuer_JSONRoundTrip(t *testing.T) {
	original := ServiceAccountIssuer{
		Issuer:    "https://oidc.example.com/id/ABC123",
		Audiences: []string{"sts.amazonaws.com", "k8s.example.com"},
		JWKSURI:   "https://oidc.example.com/.well-known/jwks.json",
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ServiceAccountIssuer
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestServiceAccountIssuer_OmittedFields(t *testing.T) {
	// Minimal issuer — audiences and jwksURI omitted should marshal without those keys
	issuer := ServiceAccountIssuer{
		Issuer: "https://oidc.example.com",
	}
	data, err := json.Marshal(issuer)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "issuer")
	assert.NotContains(t, raw, "audiences")
	assert.NotContains(t, raw, "jwksURI")
}

func TestKubeadmClusterConfigSpec_ServiceAccountIssuer_NilByDefault(t *testing.T) {
	spec := KubeadmClusterConfigSpec{}
	assert.Nil(t, spec.ServiceAccountIssuer)
}
