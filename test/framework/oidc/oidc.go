// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/big"
	"net"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	authv1client "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/rest"
	netutils "k8s.io/utils/net"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	k8sclient "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
)

const (
	// DefaultNamespace is the namespace used for the in-cluster mock OIDC server.
	DefaultNamespace = "oidc-mock"

	// ServiceName is the Service and Deployment name for the mock OIDC server.
	ServiceName = "mock-oidc"

	// DefaultKubeadmServiceCIDR is kubeadm's default Service subnet when ClusterNetwork omits one.
	DefaultKubeadmServiceCIDR = "10.96.0.0/12"

	// staticServiceIPOffset selects the 12th IP in the Service CIDR. It sits inside Kubernetes's
	// KEP-3070 static Service IP band ([1..min(max(16, size/16), 256)]) so dynamic ClusterIP
	// allocation will not claim it during cluster bootstrap, while avoiding .1 (kubernetes),
	// .10 (kube-dns), and .20 (CAREN registry addon).
	staticServiceIPOffset = 12

	mockOIDCConfigVolume   = "config"
	mockOIDCCertVolume     = "certs"
	mockOIDCConfigMapName  = "mock-oidc-config"
	mockOIDCCertSecretName = "mock-oidc-certs" //nolint:gosec // test helper constant
	jwksKey                = "keys"
	jwksKeyID              = "e2e"
	jwtSigningAlg          = "RS256"
	yamlStringTag          = "!!str"
)

// Bundle holds pre-generated TLS and JWT signing material for an in-cluster mock OIDC provider
// bound to a deterministic static Service ClusterIP.
type Bundle struct {
	ClusterIP     string
	IssuerURL     string
	DiscoveryURL  string
	CAPEM         string
	ServerCertPEM []byte
	ServerKeyPEM  []byte
	PrivateKey    *rsa.PrivateKey
}

// ExpectedUserInfo describes the authenticated user attributes expected from SelfSubjectReview.
type ExpectedUserInfo struct {
	Username string
	Groups   []string
	Extra    map[string][]string
}

// NewBundle pre-generates a self-signed CA, server certificate with IP SANs for clusterIP,
// and an RSA-2048 JWT signing keypair so clusterConfig.authentication can be injected before
// workload cluster provisioning begins.
func NewBundle(clusterIP string) (*Bundle, error) {
	parsedIP := net.ParseIP(clusterIP)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid mock OIDC Service clusterIP %q", clusterIP)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating RSA signing key: %w", err)
	}

	caCert, caPEM, caKey, err := generateCA()
	if err != nil {
		return nil, fmt.Errorf("generating mock OIDC CA: %w", err)
	}

	serverCertPEM, serverKeyPEM, err := generateServerCert(caCert, caKey, parsedIP)
	if err != nil {
		return nil, fmt.Errorf("generating mock OIDC server certificate: %w", err)
	}

	urlHost := clusterIP
	if strings.Contains(clusterIP, ":") {
		urlHost = "[" + clusterIP + "]"
	}
	issuerURL := "https://" + urlHost

	return &Bundle{
		ClusterIP:     clusterIP,
		IssuerURL:     issuerURL,
		DiscoveryURL:  issuerURL + "/.well-known/openid-configuration",
		CAPEM:         string(caPEM),
		ServerCertPEM: serverCertPEM,
		ServerKeyPEM:  serverKeyPEM,
		PrivateKey:    privateKey,
	}, nil
}

// AuthenticationConfig returns the CAREN Authentication variable pointing at this Bundle.
func (b *Bundle) AuthenticationConfig() v1alpha1.Authentication {
	return v1alpha1.Authentication{
		JWT: []v1alpha1.JWTAuthenticator{
			{
				Issuer: v1alpha1.Issuer{
					URL:                  b.IssuerURL,
					DiscoveryURL:         b.DiscoveryURL,
					Audiences:            []string{"e2e-aud"},
					AudienceMatchPolicy:  v1alpha1.AudienceMatchPolicyMatchAny,
					CertificateAuthority: b.CAPEM,
				},
				ClaimMappings: v1alpha1.ClaimMappings{
					Username: v1alpha1.PrefixedClaimOrExpression{
						Claim:  "email",
						Prefix: new(""),
					},
					Groups: &v1alpha1.PrefixedClaimOrExpression{
						Claim:  "groups",
						Prefix: new(""),
					},
					Extra: []v1alpha1.ExtraMapping{
						{
							Key:             "example.org/foo",
							ValueExpression: "claims.foo",
						},
					},
				},
				ClaimValidationRules: []v1alpha1.ClaimValidationRule{
					{Claim: "hd", RequiredValue: "example.com"},
				},
				UserValidationRules: []v1alpha1.UserValidationRule{
					{
						Expression: "!user.username.startsWith('system:')",
						Message:    "system: prefix reserved",
					},
				},
			},
		},
	}
}

// MintJWT signs a compact RS256 JWT using the Go standard library.
func (b *Bundle) MintJWT(claims map[string]any) (string, error) {
	payload := make(map[string]any, len(claims)+3)
	maps.Copy(payload, claims)
	now := time.Now()
	if _, ok := payload["iss"]; !ok {
		payload["iss"] = b.IssuerURL
	}
	if _, ok := payload["iat"]; !ok {
		payload["iat"] = now.Unix()
	}
	if _, ok := payload["exp"]; !ok {
		payload["exp"] = now.Add(time.Hour).Unix()
	}

	headerJSON, _ := json.Marshal(map[string]string{
		"alg": jwtSigningAlg,
		"kid": jwksKeyID,
		"typ": "JWT",
	})
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshalling JWT claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, b.PrivateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Deploy applies the Namespace, TLS Secret, nginx ConfigMap, static-ClusterIP Service, and
// Deployment for the mock OIDC provider in the workload cluster.
func (b *Bundle) Deploy(ctx context.Context, workloadClient client.Client, namespace string) error {
	if namespace == "" {
		namespace = DefaultNamespace
	}

	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if err := k8sclient.ServerSideApply(ctx, workloadClient, ns, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("applying namespace: %w", err)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockOIDCCertSecretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       b.ServerCertPEM,
			corev1.TLSPrivateKeyKey: b.ServerKeyPEM,
		},
	}
	if err := k8sclient.ServerSideApply(ctx, workloadClient, secret, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("applying TLS secret: %w", err)
	}

	jwksJSON, err := buildJWKS(b.PrivateKey)
	if err != nil {
		return err
	}
	discoveryJSON, err := buildDiscoveryJSON(b.IssuerURL)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockOIDCConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"default.conf":         buildNginxConf(),
			"openid-configuration": discoveryJSON,
			jwksKey:                jwksJSON,
		},
	}
	if err := k8sclient.ServerSideApply(ctx, workloadClient, cm, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("applying configmap: %w", err)
	}

	labels := map[string]string{"app": ServiceName}
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: b.ClusterIP,
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Port:       443,
					TargetPort: intstr.FromInt32(8443),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if err := k8sclient.ServerSideApply(ctx, workloadClient, svc, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("applying service with clusterIP %s: %w", b.ClusterIP, err)
	}

	fetchedSvc := &corev1.Service{}
	if err := workloadClient.Get(
		ctx,
		client.ObjectKey{Namespace: namespace, Name: ServiceName},
		fetchedSvc,
	); err != nil {
		return fmt.Errorf("reading back mock OIDC service: %w", err)
	}
	if fetchedSvc.Spec.ClusterIP != b.ClusterIP {
		return fmt.Errorf(
			"mock OIDC service ClusterIP = %q, want %q",
			fetchedSvc.Spec.ClusterIP,
			b.ClusterIP,
		)
	}

	replicas := int32(1)
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: namespace,
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
									MountPath: "/etc/nginx/conf.d/default.conf",
									SubPath:   "default.conf",
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
	if err := k8sclient.ServerSideApply(ctx, workloadClient, deployment, k8sclient.ForceOwnership); err != nil {
		return fmt.Errorf("applying deployment: %w", err)
	}

	return nil
}

// StaticServiceIPFromCIDR selects a deterministic static Service IP inside Kubernetes's
// KEP-3070 static ClusterIP band for cidr.
func StaticServiceIPFromCIDR(cidr string) (string, error) {
	subnets, err := netutils.ParseCIDRs([]string{cidr})
	if err != nil || len(subnets) == 0 {
		return "", fmt.Errorf("parsing service CIDR %q: %w", cidr, err)
	}
	subnet := subnets[0]
	ones, bits := subnet.Mask.Size()
	if bits-ones < 4 {
		return "", fmt.Errorf(
			"service CIDR %q prefix /%d is too small to reserve static offset %d",
			cidr, ones, staticServiceIPOffset,
		)
	}
	ip, err := netutils.GetIndexedIP(subnet, staticServiceIPOffset)
	if err != nil {
		return "", fmt.Errorf("selecting indexed Service IP from %q: %w", cidr, err)
	}
	return ip.String(), nil
}

// ResolveClusterTemplateServiceCIDR extracts the primary Service CIDR from the Cluster document
// in raw, expanding ${VAR} / ${VAR:-default} expressions with lookupVar and falling back to
// kubeadm's 10.96.0.0/12 default only when spec.clusterNetwork.services.cidrBlocks is absent.
func ResolveClusterTemplateServiceCIDR(
	raw []byte,
	lookupVar func(string) (string, bool),
) (string, error) {
	docs, err := decodeYAMLDocuments(raw)
	if err != nil {
		return "", err
	}
	for _, doc := range docs {
		root := clusterMappingRoot(doc)
		if root == nil {
			continue
		}
		cidrs := mappingValue(
			mappingValue(mappingValue(mappingValue(root, "spec"), "clusterNetwork"), "services"),
			"cidrBlocks",
		)
		if cidrs == nil || cidrs.Kind != yaml.SequenceNode || len(cidrs.Content) == 0 {
			return DefaultKubeadmServiceCIDR, nil
		}
		expanded := expandTemplateVar(cidrs.Content[0].Value, lookupVar)
		subnets, err := netutils.ParseCIDRs([]string{expanded})
		if err != nil || len(subnets) == 0 {
			return "", fmt.Errorf(
				"resolved service CIDR %q (from %q) is invalid: %w",
				expanded, cidrs.Content[0].Value, err,
			)
		}
		return subnets[0].String(), nil
	}
	return "", errors.New("cluster template contains no Cluster document")
}

// VerifyServiceIPInCluster checks that ip matches the static Service IP derived from the live
// Cluster's spec.clusterNetwork.services.cidrBlocks.
func VerifyServiceIPInCluster(ip string, cluster *clusterv1.Cluster) error {
	if cluster == nil {
		return errors.New("cluster is nil")
	}
	cidrs := cluster.Spec.ClusterNetwork.Services.CIDRBlocks
	if len(cidrs) == 0 {
		cidrs = []string{DefaultKubeadmServiceCIDR}
	}
	expected, err := StaticServiceIPFromCIDR(cidrs[0])
	if err != nil {
		return err
	}
	if ip != expected {
		return fmt.Errorf(
			"mock OIDC ClusterIP %q does not match static IP %q for cluster service CIDRs %v",
			ip, expected, cidrs,
		)
	}
	return nil
}

// EnableAuthenticationInClusterTemplate sets clusterConfig.authentication in the Cluster
// document's topology variables while preserving multi-document YAML (e.g. Nutanix Secrets)
// and quoted envsubst placeholders.
func EnableAuthenticationInClusterTemplate(
	raw []byte,
	auth v1alpha1.Authentication,
) ([]byte, error) {
	docs, err := decodeYAMLDocuments(raw)
	if err != nil {
		return nil, err
	}

	authYAML, err := sigsyaml.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("marshalling authentication config: %w", err)
	}
	var authDoc yaml.Node
	if err := yaml.Unmarshal(authYAML, &authDoc); err != nil {
		return nil, fmt.Errorf("decoding authentication yaml node: %w", err)
	}
	if authDoc.Kind != yaml.DocumentNode || len(authDoc.Content) == 0 {
		return nil, errors.New("marshalled authentication produced empty YAML node")
	}
	authNode := authDoc.Content[0]

	patched := false
	for _, doc := range docs {
		root := clusterMappingRoot(doc)
		if root == nil {
			continue
		}
		topology := mappingValue(mappingValue(root, "spec"), "topology")
		if topology == nil {
			return nil, errors.New("cluster template has no spec.topology")
		}
		vars := mappingValue(topology, "variables")
		if vars == nil {
			vars = &yaml.Node{Kind: yaml.SequenceNode}
			setMappingValue(topology, "variables", vars)
		}
		upsertClusterConfigAuthenticationNode(vars, authNode)
		patched = true
	}
	if !patched {
		return nil, errors.New("cluster template contains no Cluster document")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, doc := range docs {
		if err := enc.Encode(doc); err != nil {
			return nil, fmt.Errorf("encoding patched cluster template: %w", err)
		}
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing yaml encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// ClusterControlPlaneSettled reports whether a v1beta2 Cluster has finished reconciling its
// control plane without an active rollout.
func ClusterControlPlaneSettled(c *clusterv1.Cluster) bool {
	conds := c.Status.Conditions
	return apimeta.IsStatusConditionTrue(conds, clusterv1.ClusterAvailableCondition) &&
		apimeta.IsStatusConditionTrue(conds, clusterv1.ClusterControlPlaneMachinesUpToDateCondition) &&
		!apimeta.IsStatusConditionTrue(conds, clusterv1.ClusterRollingOutCondition)
}

// KubeadmControlPlaneAtReplicas reports whether spec, observed, and ready replica counts equal want.
func KubeadmControlPlaneAtReplicas(kcp *controlplanev1.KubeadmControlPlane, want int32) bool {
	if kcp.Spec.Replicas == nil || *kcp.Spec.Replicas != want {
		return false
	}
	if kcp.Status.Replicas == nil || *kcp.Status.Replicas != want {
		return false
	}
	if kcp.Status.ReadyReplicas == nil || *kcp.Status.ReadyReplicas != want {
		return false
	}
	if kcp.Status.UpToDateReplicas != nil && *kcp.Status.UpToDateReplicas != want {
		return false
	}
	return true
}

// KubeadmControlPlaneHasFile reports whether kcp's KubeadmConfigSpec includes a file at path.
func KubeadmControlPlaneHasFile(kcp *controlplanev1.KubeadmControlPlane, path string) bool {
	for i := range kcp.Spec.KubeadmConfigSpec.Files {
		if kcp.Spec.KubeadmConfigSpec.Files[i].Path == path {
			return true
		}
	}
	return false
}

// AuthenticationConfigCurrent reports whether kcp has observed its current generation, has all
// desired control-plane replicas ready and up-to-date, and renders /etc/kubernetes/authentication.yaml.
func AuthenticationConfigCurrent(kcp *controlplanev1.KubeadmControlPlane) bool {
	if kcp.Spec.Replicas == nil || *kcp.Spec.Replicas <= 0 {
		return false
	}
	return kcp.Generation > 0 &&
		kcp.Status.ObservedGeneration == kcp.Generation &&
		KubeadmControlPlaneAtReplicas(kcp, *kcp.Spec.Replicas) &&
		KubeadmControlPlaneHasFile(kcp, authentication.AuthenticationConfigPath)
}

// BearerOnlyConfig returns a shallow copy of cfg that authenticates solely with token, stripping
// admin client certificates or static credentials that would otherwise take precedence.
func BearerOnlyConfig(cfg *rest.Config, token string) *rest.Config {
	out := rest.CopyConfig(cfg)
	out.BearerToken = token
	out.BearerTokenFile = ""
	out.CertFile = ""
	out.KeyFile = ""
	out.CertData = nil
	out.KeyData = nil
	out.Username = ""
	out.Password = ""
	out.AuthProvider = nil
	out.ExecProvider = nil
	out.Impersonate = rest.ImpersonationConfig{}
	return out
}

// CheckUserInfo verifies that got matches the expected username, non-system groups, and extra keys.
func CheckUserInfo(got authenticationv1.UserInfo, want *ExpectedUserInfo) error {
	if want == nil {
		return errors.New("expected user info is nil")
	}
	if got.Username != want.Username {
		return fmt.Errorf("authenticated username = %q, want %q", got.Username, want.Username)
	}
	if want.Groups != nil {
		var nonSystemGroups []string
		for _, g := range got.Groups {
			if g == "system:authenticated" {
				continue
			}
			nonSystemGroups = append(nonSystemGroups, g)
		}
		if !slices.Equal(nonSystemGroups, want.Groups) {
			return fmt.Errorf(
				"authenticated groups = %v (raw %v), want %v",
				nonSystemGroups, got.Groups, want.Groups,
			)
		}
	}
	for k, wantVals := range want.Extra {
		gotVals := []string(got.Extra[k])
		if !slices.Equal(gotVals, wantVals) {
			return fmt.Errorf(
				"authenticated extra[%q] = %v, want %v",
				k, gotVals, wantVals,
			)
		}
	}
	return nil
}

// ValidateTokenAuth submits a SelfSubjectReview authenticated solely with token against restCfg.
// When want is non-nil, authentication must succeed with matching username, groups, and extra.
// When want is nil, authentication must fail with HTTP 401 Unauthorized.
func ValidateTokenAuth(
	ctx context.Context,
	restCfg *rest.Config,
	token string,
	want *ExpectedUserInfo,
) error {
	authClient, err := authv1client.NewForConfig(BearerOnlyConfig(restCfg, token))
	if err != nil {
		return err
	}
	review, err := authClient.SelfSubjectReviews().Create(
		ctx,
		&authenticationv1.SelfSubjectReview{},
		metav1.CreateOptions{},
	)
	if want != nil {
		if err != nil {
			return fmt.Errorf("expected token to authenticate: %w", err)
		}
		return CheckUserInfo(review.Status.UserInfo, want)
	}
	if err == nil {
		return fmt.Errorf(
			"expected token to be rejected but authenticated as %q",
			review.Status.UserInfo.Username,
		)
	}
	if apierrors.IsUnauthorized(err) {
		return nil
	}
	return fmt.Errorf("expected Unauthorized, got: %w", err)
}

func buildJWKS(privateKey *rsa.PrivateKey) (string, error) {
	n := base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes())
	jwks := map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"kid": jwksKeyID,
				"alg": jwtSigningAlg,
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
	discovery := map[string]any{
		"issuer":                                issuerURL,
		"jwks_uri":                              issuerURL + "/keys",
		"response_types_supported":              []string{"id_token"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{jwtSigningAlg},
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
	return `server {
  listen 8443 ssl;
  listen [::]:8443 ssl;
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
	clusterIP net.IP,
) (certPEM, keyPEM []byte, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, nil, err
	}

	ips := []net.IP{net.ParseIP("127.0.0.1"), clusterIP}
	if clusterIP.To4() == nil {
		ips = append(ips, net.ParseIP("::1"))
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: ServiceName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{
			ServiceName,
			ServiceName + "." + DefaultNamespace,
			ServiceName + "." + DefaultNamespace + ".svc",
			ServiceName + "." + DefaultNamespace + ".svc.cluster.local",
		},
		IPAddresses: ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}

func decodeYAMLDocuments(raw []byte) ([]*yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("unmarshalling cluster template: %w", err)
		}
		docs = append(docs, &doc)
	}
	return docs, nil
}

func clusterMappingRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil
	}
	if kind := mappingValue(root, "kind"); kind == nil || kind.Value != "Cluster" {
		return nil
	}
	return root
}

func upsertClusterConfigAuthenticationNode(varsSeq, authNode *yaml.Node) {
	for _, item := range varsSeq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		name := mappingValue(item, "name")
		if name == nil || name.Value != v1alpha1.ClusterConfigVariableName {
			continue
		}
		value := mappingValue(item, "value")
		if value == nil {
			value = &yaml.Node{Kind: yaml.MappingNode}
			setMappingValue(item, "value", value)
		}
		value.Kind = yaml.MappingNode
		value.Tag = "!!map"
		value.Value = ""
		value.Style = 0
		setMappingValue(value, authentication.VariableName, authNode)
		return
	}

	varsSeq.Content = append(varsSeq.Content, &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "name"},
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: v1alpha1.ClusterConfigVariableName},
			{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "value"},
			{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: authentication.VariableName},
					authNode,
				},
			},
		},
	})
}

func expandTemplateVar(expr string, lookupVar func(string) (string, bool)) string {
	s := strings.TrimSpace(expr)
	if !strings.HasPrefix(s, "${") || !strings.HasSuffix(s, "}") {
		return strings.Trim(s, `"'`)
	}
	inner := s[2 : len(s)-1]
	name := inner
	defVal := ""
	for _, sep := range []string{":-", ":=", "-", "="} {
		if before, after, ok := strings.Cut(inner, sep); ok {
			name = before
			defVal = strings.Trim(after, `"'`)
			break
		}
	}
	if lookupVar != nil {
		if v, ok := lookupVar(name); ok && v != "" {
			return strings.Trim(v, `"'`)
		}
	}
	return defVal
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(
		node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: key},
		value,
	)
}
