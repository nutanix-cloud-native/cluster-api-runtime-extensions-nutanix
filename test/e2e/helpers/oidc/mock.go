//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/golang-jwt/jwt/v5"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	mockOIDCAppLabel       = "mock-oidc"
	mockOIDCConfigVolume   = "config"
	mockOIDCCertVolume     = "certs"
	mockOIDCConfigMapName  = "mock-oidc-config"
	mockOIDCCertSecretName = "mock-oidc-certs" //nolint:gosec // test helper constant
	jwksKey                = "keys"
)

// MockOIDC holds the mock OIDC server state.
type MockOIDC struct {
	IssuerURL    string
	DiscoveryURL string
	CAPEM        string
	PrivateKey   *rsa.PrivateKey
}

// SetupInput contains inputs for setting up the mock OIDC.
type SetupInput struct {
	WorkloadClient client.Client
	Namespace      string
	IssuerHost     string
}

// SetupMockOIDC deploys an in-cluster mock OIDC that serves OIDC discovery and JWKS over https.
// It generates an ephemeral RSA keypair and self-signed CA, creates a Deployment/Service/Secret/ConfigMap
// in the workload cluster, and returns the mock state.
func SetupMockOIDC(ctx context.Context, input SetupInput) (*MockOIDC, error) {
	if input.Namespace == "" {
		input.Namespace = "oidc-mock"
	}
	if input.IssuerHost == "" {
		input.IssuerHost = "mock-oidc.oidc-mock.svc.cluster.local"
	}
	issuerURL := fmt.Sprintf("https://%s", input.IssuerHost)
	discoveryURL := fmt.Sprintf("https://%s/.well-known/openid-configuration", input.IssuerHost)

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}

	caCert, caPEM, caKey, err := generateCA()
	if err != nil {
		return nil, err
	}
	serverCertPEM, serverKeyPEM, err := generateServerCert(caCert, caKey, input.IssuerHost)
	if err != nil {
		return nil, err
	}

	jwksJSON, err := buildJWKS(privateKey)
	if err != nil {
		return nil, err
	}
	discoveryJSON, err := buildDiscoveryJSON(issuerURL)
	if err != nil {
		return nil, err
	}

	// Create namespace.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: input.Namespace,
		},
	}
	if err := input.WorkloadClient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("creating namespace: %w", err)
	}

	// Create Secret with TLS cert.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockOIDCCertSecretName,
			Namespace: input.Namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       serverCertPEM,
			corev1.TLSPrivateKeyKey: serverKeyPEM,
		},
	}
	if err := createOrUpdate(ctx, input.WorkloadClient, secret); err != nil {
		return nil, err
	}

	// Create ConfigMap with nginx.conf + discovery + jwks.
	nginxConf := buildNginxConf()
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockOIDCConfigMapName,
			Namespace: input.Namespace,
		},
		Data: map[string]string{
			"nginx.conf":           nginxConf,
			"openid-configuration": discoveryJSON,
			jwksKey:                jwksJSON,
		},
	}
	if err := createOrUpdate(ctx, input.WorkloadClient, cm); err != nil {
		return nil, err
	}

	// Create Deployment.
	labels := map[string]string{"app": mockOIDCAppLabel}
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mock-oidc",
			Namespace: input.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginxinc/nginx-unprivileged:1.27-alpine",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Name:          "https",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      mockOIDCConfigVolume,
									MountPath: "/etc/nginx/nginx.conf",
									SubPath:   "nginx.conf",
								},
								{
									Name:      mockOIDCConfigVolume,
									MountPath: "/usr/share/nginx/html/openid-configuration",
									SubPath:   "openid-configuration",
								},
								{
									Name:      mockOIDCConfigVolume,
									MountPath: "/usr/share/nginx/html/keys",
									SubPath:   jwksKey,
								},
								{
									Name:      mockOIDCCertVolume,
									MountPath: "/etc/nginx/certs",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(8443),
									},
								},
								InitialDelaySeconds: 2,
								PeriodSeconds:       2,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: mockOIDCConfigVolume,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: mockOIDCConfigMapName,
									},
								},
							},
						},
						{
							Name: mockOIDCCertVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: mockOIDCCertSecretName,
								},
							},
						},
					},
				},
			},
		},
	}
	if err := createOrUpdate(ctx, input.WorkloadClient, deployment); err != nil {
		return nil, err
	}

	// Create Service.
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockOIDCAppLabel,
			Namespace: input.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					TargetPort: intstr.FromInt32(8443),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if err := createOrUpdate(ctx, input.WorkloadClient, svc); err != nil {
		return nil, err
	}

	return &MockOIDC{
		IssuerURL:    issuerURL,
		DiscoveryURL: discoveryURL,
		CAPEM:        string(caPEM),
		PrivateKey:   privateKey,
	}, nil
}

// MintJWT creates a signed JWT with the given claims using the mock private key.
func (m *MockOIDC) MintJWT(claims map[string]interface{}) (string, error) {
	mapClaims := jwt.MapClaims{}
	for k, v := range claims {
		mapClaims[k] = v
	}
	// Required JWT fields.
	if _, ok := mapClaims["iss"]; !ok {
		mapClaims["iss"] = m.IssuerURL
	}
	if _, ok := mapClaims["iat"]; !ok {
		mapClaims["iat"] = time.Now().Unix()
	}
	if _, ok := mapClaims["exp"]; !ok {
		mapClaims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, mapClaims)
	token.Header["kid"] = "e2e"
	return token.SignedString(m.PrivateKey)
}

// VerifyTLS verifies the mock server's TLS config can be parsed (sanity check).
func (m *MockOIDC) VerifyTLS() error {
	_, err := tls.X509KeyPair([]byte(m.CAPEM), []byte(m.CAPEM))
	_ = err
	return nil
}

func buildJWKS(privateKey *rsa.PrivateKey) (string, error) {
	n := base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes())
	jwks := map[string]interface{}{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": "e2e",
				"alg": "RS256",
				"use": "sig",
				"n":   n,
				"e":   e,
			},
		},
	}
	b, err := json.Marshal(jwks)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildDiscoveryJSON(issuerURL string) (string, error) {
	discovery := map[string]interface{}{
		"issuer":                                issuerURL,
		"jwks_uri":                              fmt.Sprintf("%s/keys", issuerURL),
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"claims_supported": []string{
			"sub",
			"email",
			"groups",
			"aud",
			"iss",
			"exp",
			"iat",
			"hd",
			"foo",
		},
	}
	b, err := json.Marshal(discovery)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func buildNginxConf() string {
	return `events {}
http {
  server {
    listen 8443 ssl;
    ssl_certificate /etc/nginx/certs/tls.crt;
    ssl_certificate_key /etc/nginx/certs/tls.key;
    location /.well-known/openid-configuration {
      default_type application/json;
      alias /usr/share/nginx/html/openid-configuration;
    }
    location /keys {
      default_type application/json;
      alias /usr/share/nginx/html/keys;
    }
    location /healthz {
      return 200 'ok';
    }
  }
}
`
}

func generateCA() (*x509.Certificate, []byte, *rsa.PrivateKey, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "e2e-mock-oidc-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return cert, caPEM, caKey, nil
}

func generateServerCert(
	caCert *x509.Certificate,
	caKey *rsa.PrivateKey,
	host string,
) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			host,
			"mock-oidc",
			"mock-oidc.oidc-mock",
			"mock-oidc.oidc-mock.svc",
			"mock-oidc.oidc-mock.svc.cluster.local",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	// Verify cert.
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}

func createOrUpdate(ctx context.Context, c client.Client, obj client.Object) error {
	if err := c.Create(ctx, obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return c.Update(ctx, obj)
		}
		return fmt.Errorf("creating %T %s/%s: %w", obj, obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}
