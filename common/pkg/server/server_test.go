// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type namedStub struct{}

func (namedStub) Name() string { return "stub" }

func TestNewWebhookServer_ImplementsWebhookServer(t *testing.T) {
	t.Parallel()
	opts := NewServerOptions()
	opts.webhookPort = 0
	opts.webhookCertDir = t.TempDir()
	ws, err := NewWebhookServer(opts)
	require.NoError(t, err)
	require.NotNil(t, ws)
	_, ok := any(ws).(webhook.Server)
	assert.True(t, ok)
}

func TestAddHandlersAndAdmissionShareListener(t *testing.T) {
	t.Parallel()

	certDir := t.TempDir()
	require.NoError(t, writeTestServingCerts(certDir))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	opts := NewServerOptions()
	opts.webhookPort = port
	opts.webhookCertDir = certDir

	ws, err := NewWebhookServer(opts)
	require.NoError(t, err)
	require.NoError(t, AddHandlers(ws, namedStub{}))

	ws.Register("/mutate-test", &webhook.Admission{
		Handler: admission.HandlerFunc(func(_ context.Context, _ admission.Request) admission.Response {
			return admission.Allowed("")
		}),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- ws.Start(ctx) }()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test-only
		},
	}
	require.Eventually(t, func() bool {
		resp, err := client.Post(
			fmt.Sprintf("https://127.0.0.1:%d/hooks.runtime.cluster.x-k8s.io/v1alpha1/discovery", port),
			"application/json",
			nil,
		)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 50*time.Millisecond)

	resp, err := client.Post(
		fmt.Sprintf("https://127.0.0.1:%d/mutate-test", port),
		"application/json",
		nil,
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Admission may 400 on empty body; connection + TLS + route hit is enough.
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestServerOptions_NoAdmissionCertDirFlag(t *testing.T) {
	t.Parallel()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	NewServerOptions().AddFlags(fs)
	assert.Nil(t, fs.Lookup("admission-webhook-cert-dir"))
	assert.NotNil(t, fs.Lookup("webhook-port"))
	assert.NotNil(t, fs.Lookup("webhook-cert-dir"))
}

func writeTestServingCerts(dir string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	certOut, err := os.Create(filepath.Join(dir, "tls.crt"))
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	keyOut, err := os.Create(filepath.Join(dir, "tls.key"))
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
}
