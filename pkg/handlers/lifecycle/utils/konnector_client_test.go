// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync"
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
	assert.Contains(t, err.Error(), "Prism Central credentials cannot be nil")
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

func TestNewPrismCentralKonnectorClient_IsolatesDefaultTransportConcurrently(t *testing.T) {
	const clusterUUID = "11111111-2222-3333-4444-555566667777"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/karbon/v1-alpha.1/k8s/cluster-registrations/"+clusterUUID {
			_, _ = w.Write([]byte(`{"name":"test-cluster","uuid":"` + clusterUUID + `","status":"Registered"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: srv.Certificate().Raw,
	})
	trustBundle := base64.StdEncoding.EncodeToString(certPEM)
	hostPort := srv.Listener.Addr().String()

	const workers = 8
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func(idx int) {
			defer wg.Done()

			insecure := idx%2 == 0
			bundle := trustBundle
			if insecure {
				bundle = ""
			}

			pcClient, err := NewPrismCentralKonnectorClient(&prismgoclient.Credentials{
				Endpoint: hostPort,
				URL:      srv.URL,
				Username: "admin",
				Password: "secret",
				Insecure: insecure,
			}, bundle)
			if !assert.NoError(t, err) {
				return
			}

			reg, err := pcClient.GetClusterRegistration(context.Background(), clusterUUID)
			if assert.NoError(t, err) {
				assert.Equal(t, clusterUUID, reg.UUID)
			}
		}(i)
	}
	wg.Wait()

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	require.True(t, ok)
	if defaultTransport.TLSClientConfig != nil {
		assert.False(
			t,
			defaultTransport.TLSClientConfig.InsecureSkipVerify,
			"NewPrismCentralKonnectorClient must not leave InsecureSkipVerify=true on http.DefaultTransport",
		)
	}
}
