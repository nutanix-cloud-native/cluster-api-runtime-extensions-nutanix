// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type namedStub struct{}

func (namedStub) Name() string { return "stub" }

type fullHooksStub struct {
	acpiFn func(
		context.Context,
		*runtimehooksv1.AfterControlPlaneInitializedRequest,
		*runtimehooksv1.AfterControlPlaneInitializedResponse,
	)
}

func (fullHooksStub) Name() string { return "caren" }

func (fullHooksStub) BeforeClusterCreate(
	_ context.Context,
	_ *runtimehooksv1.BeforeClusterCreateRequest,
	resp *runtimehooksv1.BeforeClusterCreateResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (s fullHooksStub) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	if s.acpiFn != nil {
		s.acpiFn(ctx, req, resp)
		return
	}
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (fullHooksStub) BeforeClusterUpgrade(
	_ context.Context,
	_ *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (fullHooksStub) AfterControlPlaneUpgrade(
	_ context.Context,
	_ *runtimehooksv1.AfterControlPlaneUpgradeRequest,
	resp *runtimehooksv1.AfterControlPlaneUpgradeResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (fullHooksStub) BeforeClusterDelete(
	_ context.Context,
	_ *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (fullHooksStub) DiscoverVariables(
	_ context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (fullHooksStub) GeneratePatches(
	_ context.Context,
	_ *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

func (fullHooksStub) ValidateTopology(
	_ context.Context,
	_ *runtimehooksv1.ValidateTopologyRequest,
	resp *runtimehooksv1.ValidateTopologyResponse,
) {
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

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

func TestAddHandlers_LifecycleTimeoutSecondsAndBoundedContext(t *testing.T) {
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

	stub := fullHooksStub{
		acpiFn: func(
			ctx context.Context,
			_ *runtimehooksv1.AfterControlPlaneInitializedRequest,
			resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
		) {
			<-ctx.Done()
			resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
			resp.SetMessage(fmt.Sprintf("addon timed out cleanly: %v", ctx.Err()))
		},
	}
	require.NoError(t, addHandlersWithLifecycleTimeout(ws, 200*time.Millisecond, stub))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- ws.Start(ctx) }()

	httpClient := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test-only
		},
	}

	var discoveryResp runtimehooksv1.DiscoveryResponse
	require.Eventually(t, func() bool {
		resp, postErr := httpClient.Post(
			fmt.Sprintf("https://127.0.0.1:%d/hooks.runtime.cluster.x-k8s.io/v1alpha1/discovery", port),
			"application/json",
			bytes.NewReader(
				[]byte(`{"apiVersion":"hooks.runtime.cluster.x-k8s.io/v1alpha1","kind":"DiscoveryRequest"}`),
			),
		)
		if postErr != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		return json.NewDecoder(resp.Body).Decode(&discoveryResp) == nil
	}, 10*time.Second, 50*time.Millisecond)

	handlersByName := make(map[string]runtimehooksv1.ExtensionHandler, len(discoveryResp.Handlers))
	for _, h := range discoveryResp.Handlers {
		handlersByName[h.Name] = h
	}

	for _, lifecycleName := range []string{"caren-bcc", "caren-acpi", "caren-bcu", "caren-acpu", "caren-bcd"} {
		h, ok := handlersByName[lifecycleName]
		require.True(t, ok, "missing handler %s", lifecycleName)
		require.NotNil(t, h.TimeoutSeconds, "expected TimeoutSeconds to be set for %s", lifecycleName)
		assert.Equal(t, int32(30), *h.TimeoutSeconds, "unexpected TimeoutSeconds for %s", lifecycleName)
	}

	for _, mutationName := range []string{"caren-dv", "caren-gp", "caren-vt"} {
		h, ok := handlersByName[mutationName]
		require.True(t, ok, "missing handler %s", mutationName)
		assert.Nil(t, h.TimeoutSeconds, "expected nil TimeoutSeconds for mutation handler %s", mutationName)
	}

	// Verify that a blocking lifecycle handler is bounded by the server-side context deadline
	// and returns a structured HTTP 200 ResponseStatusFailure before the HTTP client timeout expires.
	acpiReq := runtimehooksv1.AfterControlPlaneInitializedRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: runtimehooksv1.GroupVersion.String(),
			Kind:       "AfterControlPlaneInitializedRequest",
		},
		Cluster: clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "default",
			},
		},
	}
	reqBytes, err := json.Marshal(acpiReq)
	require.NoError(t, err)

	resp, err := httpClient.Post(
		fmt.Sprintf(
			"https://127.0.0.1:%d/hooks.runtime.cluster.x-k8s.io/v1alpha1/aftercontrolplaneinitialized/caren-acpi",
			port,
		),
		"application/json",
		bytes.NewReader(reqBytes),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var acpiResp runtimehooksv1.AfterControlPlaneInitializedResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&acpiResp))
	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, acpiResp.Status)
	assert.Contains(t, acpiResp.Message, "addon timed out cleanly")

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
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
