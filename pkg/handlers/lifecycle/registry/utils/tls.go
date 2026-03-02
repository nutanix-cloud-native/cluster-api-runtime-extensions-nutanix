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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

const (
	caCrtKey = "ca.crt"
)

var (
	// Valid for 10 years.
	defaultRootCADuration = 10 * 365 * 24 * time.Hour

	// Similar to CAPI, set the NotBefore to a few minutes in the past to account for clock skew.
	// This cert is being generated on the management cluster, but used by a workload cluster.
	defaultCertificateNotBeforeSkew = 5 * time.Minute
	// Valid for 2 years to avoid expiring before the cluster is upgraded.
	defaultCertificateDuration = 2 * 365 * 24 * time.Hour
)

// EnsureRegistryAddonRootCASecret ensures that the registry addon root CA secret exists.
// This Secret is used to sign the registry TLS certificates for the remote clusters.
func EnsureRegistryAddonRootCASecret(
	ctx context.Context,
	c ctrlclient.Client,
) error {
	globalTLSCertificateSecret, err := handlersutils.SecretForRegistryAddonRootCA(ctx, c)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	// If the secret already exists, we don't need to do anything.
	if globalTLSCertificateSecret != nil {
		return nil
	}

	// Otherwise, create it in the management cluster namespace.
	managementCluster, err := utils.ManagementOrFutureManagementCluster(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get management cluster: %w", err)
	}

	certPEM, keyPEM, err := generateRegistryAddonRootCAData()
	if err != nil {
		return fmt.Errorf("failed to generate registry addon root CA data: %w", err)
	}
	rootCASecret := buildRegistryAddonRootCASecret(certPEM, keyPEM, managementCluster.GetNamespace())
	err = handlersutils.EnsureSecretForLocalCluster(ctx, c, rootCASecret, managementCluster)
	if err != nil {
		return fmt.Errorf("failed to ensure registry addon root CA secret: %w", err)
	}

	return nil
}

func generateRegistryAddonRootCAData() (certPEM, keyPEM []byte, err error) {
	// 1. generate a new RSA private key.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// 2. create a self-signed CA certificate.
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "registry-addon",
		},
		NotBefore:             time.Now().Add(-1 * defaultCertificateNotBeforeSkew),
		NotAfter:              time.Now().Add(defaultRootCADuration),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return certPEM, keyPEM, nil
}

func buildRegistryAddonRootCASecret(
	certPEM,
	keyPEM []byte,
	namespace string,
) *corev1.Secret {
	data := map[string][]byte{
		caCrtKey:                certPEM,
		corev1.TLSCertKey:       certPEM,
		corev1.TLSPrivateKeyKey: keyPEM,
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      handlersutils.RegistryAddonRootCASecretName,
			Namespace: namespace,
		},
		Data: data,
	}
}

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
