// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
)

func TestNewPrismCentralKonnectorClient_NilCredentials(t *testing.T) {
	// Negative test case: nil credentials should return error
	client, err := NewPrismCentralKonnectorClient(nil, "")

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "prism central credentials cannot be nil")
}

func TestNewPrismCentralKonnectorClient_WithTrustBundle(t *testing.T) {
	// Positive test case: valid credentials with trust bundle
	credentials := &prismgoclient.Credentials{
		Endpoint: "test.example.com:9440",
		URL:      "https://test.example.com:9440",
		Username: "testuser",
		Password: "testpass",
		Insecure: true,
		Port:     "9440",
	}

	// Create a valid base64-encoded certificate
	cert := "-----BEGIN CERTIFICATE-----\nTEST\n-----END CERTIFICATE-----"
	trustBundle := base64.StdEncoding.EncodeToString([]byte(cert))

	// Note: This will fail at actual client creation since we don't have a real PC,
	// but it tests the trust bundle decoding logic
	client, err := NewPrismCentralKonnectorClient(credentials, trustBundle)

	// The error should be from client creation, not from trust bundle decoding
	require.Error(t, err)
	assert.Nil(t, client)
	// Should not be a base64 decode error
	assert.NotContains(t, err.Error(), "failed to decode base64 certificate")
	// Should have set insecure to false when trust bundle is provided
	assert.False(t, credentials.Insecure)
}
