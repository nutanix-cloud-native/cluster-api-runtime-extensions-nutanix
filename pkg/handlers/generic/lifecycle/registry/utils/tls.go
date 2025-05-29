// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"cmp"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	caCrtKey = "ca.crt"
)

var (
	// Similar to CAPI, set the NotBefore to a few minutes in the past to account for clock skew.
	// This cert is being generated on the management cluster, but used by a workload cluster.
	defaultCertificateNotBeforeSkew = 5 * time.Minute
	// Valid for 2 years to avoid expiring before the cluster is upgraded.
	defaultCertificateDuration = 2 * 365 * 24 * time.Hour
)

type EnsureCertificateOpts struct {
	// RemoteSecretKey is the name and namespace of the TLS secret to be created on the remote cluster.
	RemoteSecretKey ctrlclient.ObjectKey

	Spec CertificateSpec
}

type CertificateSpec struct {
	// CommonName is the common name to be included in the certificate.
	CommonName string
	// DNSNames is a list of DNS names to be included in the certificate.
	DNSNames []string
	// IPAddresses is a list of IP addresses to be included in the certificate.
	IPAddresses []string
	// Duration is the duration for which the certificate is valid.
	Duration time.Duration
}

// EnsureCASecretForCluster ensures that the registry addon CA secret exists for the given cluster.
// It copies the ca.crt value from the global CA secret to a unique secret in the cluster's namespace.
func EnsureCASecretForCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) error {
	globalTLSCertificateSecret, err := handlersutils.SecretForRegistryAddonRootCA(ctx, c)
	if err != nil {
		return err
	}

	clusterCASecret := buildClusterCASecret(globalTLSCertificateSecret, cluster)

	// Copy the global CA certificate to a cluster CA secret.
	err = handlersutils.EnsureSecretForLocalCluster(ctx, c, clusterCASecret, cluster)
	if err != nil {
		return fmt.Errorf("failed to ensure cluster CA secret for cluster: %w", err)
	}

	return nil
}

// EnsureRegistryServerCertificateSecretOnRemoteCluster ensures that a registry TLS certificate is signed
// by the global CA and is created as secret on the remote cluster.
//
// The high level flow is as follows:
// 1. Create a new TLS certificate and sign it with the global CA.
// 2. Copy the TLS certificate secret to the remote cluster to be used by the registry Pods.
//
// Intentionally not using cert-manager to create the certificate,
// as we want to avoid automatic renewal and instead recreate the certificate each time with a new expiration date.
func EnsureRegistryServerCertificateSecretOnRemoteCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	opts *EnsureCertificateOpts,
) error {
	globalTLSCertificateSecret, err := handlersutils.SecretForRegistryAddonRootCA(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get TLS secret used to sign the certificate: %w", err)
	}

	// Always recreate the TLS certificate using the global CA to sign it.
	certPEM, keyPEM, caPEM, err := generateCertificateData(globalTLSCertificateSecret, opts)
	if err != nil {
		return fmt.Errorf("failed to generate new certificate: %w", err)
	}
	err = copyTLSCertificateSecretToRemoteCluster(
		ctx,
		c,
		cluster,
		opts.RemoteSecretKey,
		certPEM, keyPEM, caPEM,
	)
	if err != nil {
		return err
	}

	return nil
}

func buildClusterCASecret(
	globalTLSCertificateSecret *corev1.Secret,
	cluster *clusterv1.Cluster,
) *corev1.Secret {
	// The root CA will have, tls.crt, tls.key, and ca.crt.
	// We only copy the ca.crt from the global TLS secret to the cluster CA secret.
	data := map[string][]byte{
		caCrtKey: globalTLSCertificateSecret.Data[caCrtKey],
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      handlersutils.SecretNameForRegistryAddonCA(cluster),
			Namespace: cluster.Namespace,
		},
		Data: data,
	}
}

func generateCertificateData(
	globalCASecret *corev1.Secret,
	opts *EnsureCertificateOpts,
) (serverCertPEM, serverKeyPEM, caCertPEM []byte, err error) {
	// 1. load CA PEMs from Secret
	caCertPEM, ok := globalCASecret.Data[corev1.TLSCertKey]
	if !ok {
		return nil, nil, nil,
			fmt.Errorf("%s not found in Secret", corev1.TLSCertKey)
	}
	caKeyPEM, ok := globalCASecret.Data[corev1.TLSPrivateKeyKey]
	if !ok {
		return nil, nil, nil,
			fmt.Errorf("%s not found in Secret", corev1.TLSPrivateKeyKey)
	}

	// 2. parse CA cert
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil || caBlock.Type != "CERTIFICATE" {
		return nil, nil, nil, fmt.Errorf("failed to decode CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse CA cert: %w", err)
	}

	// 3. parse CA private key (PKCS#1 or PKCS#8)
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		return nil, nil, nil, fmt.Errorf("failed to decode CA private key PEM")
	}
	var caPriv interface{}
	switch keyBlock.Type {
	case "RSA PRIVATE KEY":
		caPriv, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	case "PRIVATE KEY":
		caPriv, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	default:
		err = fmt.Errorf("unsupported private key encoding type %q", keyBlock.Type)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	// 4. generate server key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	// 5. build server cert template
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: opts.Spec.CommonName,
		},
		NotBefore:             time.Now().Add(-1 * defaultCertificateNotBeforeSkew),
		NotAfter:              time.Now().Add(cmp.Or(opts.Spec.Duration, defaultCertificateDuration)),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              opts.Spec.DNSNames,
		BasicConstraintsValid: true,
	}
	for _, s := range opts.Spec.IPAddresses {
		if ip := net.ParseIP(s); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		}
	}

	// 6. sign server cert with the CA
	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &serverKey.PublicKey, caPriv)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// 7. PEM-encode outputs
	serverCertPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	serverKeyPEM = pem.EncodeToMemory(
		&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey)},
	)

	return serverCertPEM, serverKeyPEM, caCertPEM, nil
}

// copyTLSCertificateSecretToRemoteCluster copies the registry TLS certificate Secret to the remote cluster.
func copyTLSCertificateSecretToRemoteCluster(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
	key ctrlclient.ObjectKey,
	certPEM, keyPEM, caPEM []byte,
) error {
	err := handlersutils.EnsureSecretOnRemoteCluster(
		ctx,
		c, buildRegistryTLSCertificateSecret(key, certPEM, keyPEM, caPEM),
		cluster,
	)
	if err != nil {
		return fmt.Errorf("failed to create registry addon TLS secret on remote cluster: %w", err)
	}

	return nil
}

func buildRegistryTLSCertificateSecret(
	key ctrlclient.ObjectKey,
	certPEM, keyPEM, caPEM []byte,
) *corev1.Secret {
	data := map[string][]byte{
		corev1.TLSCertKey:       certPEM,
		corev1.TLSPrivateKeyKey: keyPEM,
		caCrtKey:                caPEM,
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
		},
		Data: data,
		Type: corev1.SecretTypeTLS,
	}
}
