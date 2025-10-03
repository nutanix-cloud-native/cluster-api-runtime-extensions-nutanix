// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func Test_generateCertificateData(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		opts    *EnsureCertificateOpts
		wantErr error
	}{
		{
			name: "valid certificate with a set duration",
			opts: &EnsureCertificateOpts{
				Spec: CertificateSpec{
					CommonName:  "common-name",
					DNSNames:    []string{"myregistry.example.com"},
					IPAddresses: []string{"192.168.0.20"},
					Duration:    30 * 24 * time.Hour,
				},
			},
		},
		{
			name: "valid certificate with a default duration",
			opts: &EnsureCertificateOpts{
				Spec: CertificateSpec{
					CommonName:  "common-name",
					DNSNames:    []string{"myregistry.example.com"},
					IPAddresses: []string{"192.168.0.20"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			globalCASecret := testGlobalRegistryAddonTLSCertificate()
			serverCertPEM, serverKeyPEM, caCertPEM, err := generateCertificateData(globalCASecret, tt.opts)
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
			require.NotEmpty(t, serverCertPEM)
			require.NotEmpty(t, serverKeyPEM)
			require.NotEmpty(t, caCertPEM)

			assertCertificateData(t, globalCASecret, serverCertPEM, serverKeyPEM, caCertPEM, tt.opts.Spec)
		})
	}
}

func assertCertificateData(
	t *testing.T,
	globalCASecret *corev1.Secret,
	serverCertPEM, serverKeyPEM, caCertPEM []byte,
	opts CertificateSpec,
) {
	t.Helper()

	rootCACertBytes := globalCASecret.Data[caCrtKey]
	rootCACert := parseCertPEM(t, rootCACertBytes)

	assert.Equal(t, rootCACertBytes, caCertPEM)

	// Decode and parse server cert
	cert := parseCertPEM(t, serverCertPEM)
	key := parseKeyPEM(t, serverKeyPEM)

	require.NoError(t, cert.CheckSignatureFrom(rootCACert), "server cert not signed by CA")
	assert.Equal(t, opts.CommonName, cert.Subject.CommonName)
	assert.Equal(t, opts.DNSNames, cert.DNSNames)
	gotCertIPAddresses := make([]string, 0, len(opts.IPAddresses))
	for _, ipAddress := range cert.IPAddresses {
		gotCertIPAddresses = append(gotCertIPAddresses, ipAddress.String())
	}
	assert.Equal(t, opts.IPAddresses, gotCertIPAddresses)
	assert.GreaterOrEqual(t, key.N.BitLen(), 2048)

	wantDuration := opts.Duration
	if wantDuration == 0 {
		wantDuration = defaultCertificateDuration
	}
	ttl := cert.NotAfter.Sub(cert.NotBefore)
	// Assert that the duration is about what was requested
	assert.InDelta(t, wantDuration, ttl, float64(defaultCertificateNotBeforeSkew))
}

// parseCertPEM takes a PEM‐encoded cert and returns the parsed *x509.Certificate.
func parseCertPEM(
	t *testing.T,
	pemBytes []byte,
) *x509.Certificate {
	t.Helper()

	block, rest := pem.Decode(pemBytes)
	require.NotNil(t, block, "failed to decode PEM block")
	require.Equal(t, "CERTIFICATE", block.Type, "expected PEM block type to be CERTIFICATE")
	assert.Empty(t, rest, "extra data after first PEM block")

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "failed to parse certificate")

	return cert
}

func parseKeyPEM(
	t *testing.T,
	pemBytes []byte,
) *rsa.PrivateKey {
	t.Helper()

	block, rest := pem.Decode(pemBytes)
	require.NotNil(t, block, "failed to decode PEM block")
	assert.Empty(t, rest, "extra data after first PEM block")

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	require.NoError(t, err, "failed to parse private key")

	return key
}
