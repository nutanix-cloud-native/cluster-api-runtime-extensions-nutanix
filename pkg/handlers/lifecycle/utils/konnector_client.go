// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package utils provides utility functions for lifecycle handlers.
package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	prismgoclient "github.com/nutanix-cloud-native/prism-go-client"
	konnectorprismgoclient "github.com/nutanix-cloud-native/prism-go-client/konnector"
)

// prismClientInitMu serializes NewKonnectorAPIClient calls because prism-go-client mutates
// http.DefaultTransport.(*http.Transport) in place before applying WithRoundTripper:
// - https://github.com/nutanix-cloud-native/prism-go-client/blob/6020dda92b21463b4a1f17d75138ff62073b76eb/internal/client.go#L187-L189
// - https://github.com/nutanix-cloud-native/prism-go-client/blob/6020dda92b21463b4a1f17d75138ff62073b76eb/internal/client.go#L85-L104
//
//nolint:gochecknoglobals,lll // Must be package-level to serialize process-wide http.DefaultTransport mutation.
var prismClientInitMu sync.Mutex

// NewPrismCentralKonnectorClient creates a new Prism Konnector client that is used to call the Konnector APIs.
func NewPrismCentralKonnectorClient(credentials *prismgoclient.Credentials, additionalTrustBundle string,
	clientOpts ...konnectorprismgoclient.ClientOption,
) (*PrismCentralKonnectorClient, error) {
	if credentials == nil {
		return nil, fmt.Errorf(
			" Prism Central credentials cannot be nil, needed to create Prism Central Konnector client",
		)
	}

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get system cert pool: %w", err)
	}

	if additionalTrustBundle != "" {
		certBytes, decodeErr := base64.StdEncoding.DecodeString(additionalTrustBundle)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode base64 certificate: %w", decodeErr)
		}
		// Set insecure to false if trust bundle is provided.
		credentials.Insecure = false
		clientOpts = append(clientOpts, konnectorprismgoclient.WithPEMEncodedCertBundle(certBytes))
		if parseErr := appendPEMCerts(certPool, certBytes); parseErr != nil {
			return nil, parseErr
		}
	}

	var proxyFunc func(*http.Request) (*url.URL, error)
	if credentials.ProxyURL != "" {
		proxyURL, proxyErr := url.Parse(credentials.ProxyURL)
		if proxyErr != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", proxyErr)
		}
		proxyFunc = http.ProxyURL(proxyURL)
	}

	prismCentralKonnectorClient, err := newIsolatedKonnectorAPIClient(
		*credentials,
		certPool,
		proxyFunc,
		clientOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create prism konnector client: %w", err)
	}

	return &PrismCentralKonnectorClient{prismCentralKonnectorClient: prismCentralKonnectorClient}, nil
}

func newIsolatedKonnectorAPIClient(
	credentials prismgoclient.Credentials, //nolint:gocritic // Matches NewKonnectorAPIClient value signature.
	certPool *x509.CertPool,
	proxyFunc func(*http.Request) (*url.URL, error),
	clientOpts ...konnectorprismgoclient.ClientOption,
) (*konnectorprismgoclient.Client, error) {
	prismClientInitMu.Lock()
	defer prismClientInitMu.Unlock()

	var (
		isolatedTransport *http.Transport
		defaultTransport  *http.Transport
		origTLSConfig     *tls.Config
		origProxy         func(*http.Request) (*url.URL, error)
	)
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		defaultTransport = dt
		if dt.TLSClientConfig != nil {
			origTLSConfig = dt.TLSClientConfig.Clone()
		}
		origProxy = dt.Proxy
		isolatedTransport = dt.Clone()
		defer func() {
			defaultTransport.TLSClientConfig = origTLSConfig
			defaultTransport.Proxy = origProxy
		}()
	} else {
		isolatedTransport = &http.Transport{}
	}

	isolatedTransport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            certPool,
		InsecureSkipVerify: credentials.Insecure, //nolint:gosec // Controlled by PrismCentralEndpoint.Insecure.
	}
	if proxyFunc != nil {
		isolatedTransport.Proxy = proxyFunc
	}

	// Prepend so caller-supplied WithRoundTripper clientOpts (e.g. in tests) still win.
	allOpts := make([]konnectorprismgoclient.ClientOption, 0, len(clientOpts)+1)
	allOpts = append(allOpts, konnectorprismgoclient.WithRoundTripper(isolatedTransport))
	allOpts = append(allOpts, clientOpts...)

	return konnectorprismgoclient.NewKonnectorAPIClient(credentials, allOpts...)
}

func appendPEMCerts(certPool *x509.CertPool, certBundle []byte) error {
	for block, rest := pem.Decode(certBundle); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			return fmt.Errorf("unexpected PEM block type %q: was expecting CERTIFICATE", block.Type)
		}
		certs, err := x509.ParseCertificates(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
		for _, cert := range certs {
			certPool.AddCert(cert)
		}
	}
	return nil
}

// PrismCentralKonnectorClient wraps the Prism Central Konnector client.
type PrismCentralKonnectorClient struct {
	prismCentralKonnectorClient *konnectorprismgoclient.Client
}

// GetClusterRegistration retrieves the cluster registration from Prism Central.
func (pc *PrismCentralKonnectorClient) GetClusterRegistration(
	ctx context.Context,
	clusterUUID string,
) (*konnectorprismgoclient.K8sClusterRegistration, error) {
	if pc == nil || pc.prismCentralKonnectorClient == nil {
		return nil, fmt.Errorf("could not connect to API server on PC: client is nil")
	}
	clusterRegistration, err := pc.prismCentralKonnectorClient.ClusterRegistrationOperations.GetK8sRegistration(
		ctx,
		clusterUUID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes cluster(%s) registration: %w", clusterUUID, err)
	}
	return clusterRegistration, nil
}

// DeleteClusterRegistrationKubeconfig deletes the uploaded cluster kubeconfig from Prism Central.
func (pc *PrismCentralKonnectorClient) DeleteClusterRegistrationKubeconfig(
	ctx context.Context,
	clusterUUID string,
	force bool,
) error {
	if pc == nil || pc.prismCentralKonnectorClient == nil {
		return fmt.Errorf("could not connect to API server on PC: client is nil")
	}
	_, err := pc.prismCentralKonnectorClient.ClusterRegistrationOperations.DeleteK8sRegistrationKubeconfig(
		ctx,
		clusterUUID,
		konnectorprismgoclient.DeleteK8sRegistrationKubeconfigParams{Force: force},
	)
	if err != nil {
		return fmt.Errorf("failed to delete Kubernetes cluster(%s) registration kubeconfig: %w", clusterUUID, err)
	}
	return nil
}

// DeleteClusterRegistration deletes the cluster registration from Prism Central.
func (pc *PrismCentralKonnectorClient) DeleteClusterRegistration(
	ctx context.Context,
	clusterUUID string,
	force bool,
) error {
	if pc == nil || pc.prismCentralKonnectorClient == nil {
		return fmt.Errorf("could not connect to API server on PC: client is nil")
	}
	_, err := pc.prismCentralKonnectorClient.ClusterRegistrationOperations.DeleteK8sRegistration(
		ctx,
		clusterUUID,
		konnectorprismgoclient.DeleteK8sRegistrationParams{Force: force},
	)
	if err != nil {
		return fmt.Errorf("failed to delete Kubernetes cluster(%s) registration: %w", clusterUUID, err)
	}
	return nil
}
