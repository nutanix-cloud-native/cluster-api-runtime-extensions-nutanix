// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer"
)

const (
	// DefaultServiceAccountIssuerURL is the synthetic HTTPS ServiceAccount issuer URL used in e2e tests.
	DefaultServiceAccountIssuerURL = "https://sa-issuer.e2e.caren.nutanix.com"

	// DefaultServiceAccountJWKSURI is the synthetic JWKS URI override advertised in discovery during e2e tests.
	DefaultServiceAccountJWKSURI = "https://jwks.e2e.caren.nutanix.com/openid/v1/jwks"

	// DefaultServiceAccountAudience is the custom workload-identity audience configured in e2e tests.
	DefaultServiceAccountAudience = "sts.e2e.caren.nutanix.com"

	// DefaultKubernetesServiceAccountAudience preserves kubeadm's default API audience alongside custom audiences.
	DefaultKubernetesServiceAccountAudience = "https://kubernetes.default.svc.cluster.local"

	// UnconfiguredServiceAccountAudience is an external audience omitted from --api-audiences to verify 401 rejection.
	UnconfiguredServiceAccountAudience = "unconfigured-rp.e2e.caren.nutanix.com"

	// DefaultServiceAccountMaxTokenExpiration is the maximum ServiceAccount token lifetime configured in e2e tests.
	DefaultServiceAccountMaxTokenExpiration = 2 * time.Hour

	// ServiceAccountIssuerE2ENamespace is the workload-cluster namespace used to mint test ServiceAccount tokens.
	ServiceAccountIssuerE2ENamespace = "sa-issuer-e2e"

	// ServiceAccountIssuerE2EName is the ServiceAccount name used to mint test tokens.
	ServiceAccountIssuerE2EName = "workload-identity-sa"
)

// OIDCDiscoveryDocument represents the subset of /.well-known/openid-configuration validated in e2e tests.
type OIDCDiscoveryDocument struct {
	Issuer                           string   `json:"issuer"`
	JWKSURI                          string   `json:"jwks_uri"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

type jwksResponse struct {
	Keys []jwkEntry `json:"keys"`
}

type jwkEntry struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// DefaultE2EServiceAccountIssuer returns a self-contained ServiceAccountIssuer configuration
// exercising issuer, unsorted audiences, jwksURI override, and maxTokenExpiration without external services.
func DefaultE2EServiceAccountIssuer() v1alpha1.ServiceAccountIssuer {
	return v1alpha1.ServiceAccountIssuer{
		Issuer: DefaultServiceAccountIssuerURL,
		// Intentionally unsorted so the e2e test also verifies alphabetical sorting into --api-audiences.
		Audiences: []string{
			DefaultServiceAccountAudience,
			DefaultKubernetesServiceAccountAudience,
		},
		JWKSURI:            DefaultServiceAccountJWKSURI,
		MaxTokenExpiration: &metav1.Duration{Duration: DefaultServiceAccountMaxTokenExpiration},
	}
}

// EnableServiceAccountIssuerInClusterTemplate sets clusterConfig.serviceAccountIssuer in the
// Cluster document's topology variables while preserving multi-document YAML and quoted placeholders.
func EnableServiceAccountIssuerInClusterTemplate(
	raw []byte,
	sai v1alpha1.ServiceAccountIssuer,
) ([]byte, error) {
	return enableClusterConfigFieldInTemplate(raw, serviceaccountissuer.VariableName, sai)
}

// ExpectedServiceAccountIssuerExtraArgs returns the exact kube-apiserver extraArgs expected for sai.
func ExpectedServiceAccountIssuerExtraArgs(sai v1alpha1.ServiceAccountIssuer) map[string]string {
	args := map[string]string{
		serviceaccountissuer.ServiceAccountIssuerArg: sai.Issuer,
	}
	if len(sai.Audiences) > 0 {
		sorted := slices.Clone(sai.Audiences)
		slices.Sort(sorted)
		args[serviceaccountissuer.APIAudiencesArg] = strings.Join(sorted, ",")
	}
	if sai.JWKSURI != "" {
		args[serviceaccountissuer.ServiceAccountJWKSURIArg] = sai.JWKSURI
	}
	if sai.MaxTokenExpiration != nil {
		args[serviceaccountissuer.ServiceAccountMaxTokenExpArg] = sai.MaxTokenExpiration.Duration.String()
	}
	return args
}

// KubeadmControlPlaneHasAPIServerArg reports whether kcp's APIServer.ExtraArgs contains name=wantValue.
func KubeadmControlPlaneHasAPIServerArg(
	kcp *controlplanev1.KubeadmControlPlane,
	name, wantValue string,
) bool {
	extraArgs := kcp.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs
	for i := range extraArgs {
		if extraArgs[i].Name == name {
			return extraArgs[i].Value != nil && *extraArgs[i].Value == wantValue
		}
	}
	return false
}

// ServiceAccountIssuerConfigCurrent reports whether kcp has observed its current generation,
// has all desired control-plane replicas ready and up-to-date, and carries all expected extraArgs for sai.
func ServiceAccountIssuerConfigCurrent(
	kcp *controlplanev1.KubeadmControlPlane,
	sai v1alpha1.ServiceAccountIssuer,
) bool {
	if kcp.Spec.Replicas == nil || *kcp.Spec.Replicas <= 0 {
		return false
	}
	if kcp.Generation <= 0 ||
		kcp.Status.ObservedGeneration != kcp.Generation ||
		!KubeadmControlPlaneAtReplicas(kcp, *kcp.Spec.Replicas) {
		return false
	}
	for name, wantVal := range ExpectedServiceAccountIssuerExtraArgs(sai) {
		if !KubeadmControlPlaneHasAPIServerArg(kcp, name, wantVal) {
			return false
		}
	}
	return true
}

// FetchOIDCDiscovery fetches /.well-known/openid-configuration from the workload cluster API server.
func FetchOIDCDiscovery(ctx context.Context, restCfg *rest.Config) (*OIDCDiscoveryDocument, error) {
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}
	raw, err := clientset.Discovery().RESTClient().Get().
		AbsPath("/.well-known/openid-configuration").
		DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching /.well-known/openid-configuration: %w", err)
	}
	var doc OIDCDiscoveryDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decoding /.well-known/openid-configuration: %w", err)
	}
	return &doc, nil
}

// VerifyOIDCDiscovery checks that doc matches the configured ServiceAccountIssuer settings.
func VerifyOIDCDiscovery(doc *OIDCDiscoveryDocument, sai v1alpha1.ServiceAccountIssuer) error {
	if doc == nil {
		return errors.New("discovery document is nil")
	}
	if doc.Issuer != sai.Issuer {
		return fmt.Errorf("discovery issuer = %q, want %q", doc.Issuer, sai.Issuer)
	}
	if sai.JWKSURI != "" {
		if doc.JWKSURI != sai.JWKSURI {
			return fmt.Errorf("discovery jwks_uri = %q, want %q", doc.JWKSURI, sai.JWKSURI)
		}
	} else if doc.JWKSURI == "" {
		return errors.New("discovery jwks_uri is empty")
	}
	if len(doc.IDTokenSigningAlgValuesSupported) == 0 {
		return errors.New("discovery id_token_signing_alg_values_supported is empty")
	}
	return nil
}

// FetchJWKSKeys fetches /openid/v1/jwks from the workload API server and parses public signing keys by kid.
func FetchJWKSKeys(ctx context.Context, restCfg *rest.Config) (map[string]crypto.PublicKey, error) {
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes clientset: %w", err)
	}
	raw, err := clientset.Discovery().RESTClient().Get().
		AbsPath("/openid/v1/jwks").
		DoRaw(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching /openid/v1/jwks: %w", err)
	}
	var jwks jwksResponse
	if err := json.Unmarshal(raw, &jwks); err != nil {
		return nil, fmt.Errorf("decoding /openid/v1/jwks: %w", err)
	}
	if len(jwks.Keys) == 0 {
		return nil, errors.New("/openid/v1/jwks returned zero keys")
	}

	keys := make(map[string]crypto.PublicKey, len(jwks.Keys))
	for i := range jwks.Keys {
		pub, err := parseJWKPublicKey(&jwks.Keys[i])
		if err != nil {
			return nil, err
		}
		keys[jwks.Keys[i].Kid] = pub
	}
	return keys, nil
}

// VerifyServiceAccountJWT cryptographically verifies a compact JWT against JWKS keys and returns its claims.
func VerifyServiceAccountJWT(
	token string,
	keys map[string]crypto.PublicKey,
) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 JWT segments, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("unmarshalling JWT header: %w", err)
	}

	pub, ok := keys[header.Kid]
	if !ok && header.Kid == "" && len(keys) == 1 {
		for _, soleKey := range keys {
			pub = soleKey
			ok = true
		}
	}
	if !ok {
		return nil, fmt.Errorf("no matching JWK found for kid %q", header.Kid)
	}

	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT signature: %w", err)
	}
	signingInput := []byte(parts[0] + "." + parts[1])
	if err := verifyJWTSignature(header.Alg, pub, signingInput, sigBytes); err != nil {
		return nil, err
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("unmarshalling JWT payload: %w", err)
	}
	return claims, nil
}

// VerifyWorkloadServiceAccountIssuer validates the workload cluster's live OIDC discovery document,
// /openid/v1/jwks signature verification on minted TokenRequest JWTs, --api-audiences acceptance/rejection
// via SelfSubjectReview, and --service-account-max-token-expiration lifetime capping.
func VerifyWorkloadServiceAccountIssuer(
	ctx context.Context,
	restCfg *rest.Config,
	sai v1alpha1.ServiceAccountIssuer,
) error {
	doc, err := FetchOIDCDiscovery(ctx, restCfg)
	if err != nil {
		return err
	}
	if err := VerifyOIDCDiscovery(doc, sai); err != nil {
		return err
	}

	keys, err := FetchJWKSKeys(ctx, restCfg)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("creating kubernetes clientset: %w", err)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ServiceAccountIssuerE2ENamespace,
		},
	}
	if _, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil &&
		!apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating namespace %s: %w", ServiceAccountIssuerE2ENamespace, err)
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountIssuerE2EName,
			Namespace: ServiceAccountIssuerE2ENamespace,
		},
	}
	if _, err := clientset.CoreV1().ServiceAccounts(ServiceAccountIssuerE2ENamespace).
		Create(ctx, sa, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating serviceaccount %s: %w", ServiceAccountIssuerE2EName, err)
	}
	defer func() {
		// Delete only the ServiceAccount so Ginkgo Eventually retries never race async Namespace termination.
		_ = clientset.CoreV1().ServiceAccounts(ServiceAccountIssuerE2ENamespace).Delete(
			ctx,
			ServiceAccountIssuerE2EName,
			metav1.DeleteOptions{},
		)
	}()

	configuredAuds := sai.Audiences
	if len(configuredAuds) == 0 {
		configuredAuds = []string{sai.Issuer}
	}
	expectedSub := serviceaccount.MakeUsername(
		ServiceAccountIssuerE2ENamespace,
		ServiceAccountIssuerE2EName,
	)
	wantSAUser := &ExpectedUserInfo{
		Username: expectedSub,
		Groups:   serviceaccount.MakeGroupNames(ServiceAccountIssuerE2ENamespace),
	}

	for _, configuredAud := range configuredAuds {
		validTok, validClaims, err := mintAndVerifySAToken(
			ctx,
			clientset,
			keys,
			[]string{configuredAud},
			600,
		)
		if err != nil {
			return fmt.Errorf("minting ServiceAccount token for configured audience %q: %w", configuredAud, err)
		}
		if err := verifyTokenClaims(validClaims, sai.Issuer, expectedSub, configuredAud); err != nil {
			return err
		}
		if err := ValidateTokenAuth(ctx, restCfg, validTok, wantSAUser); err != nil {
			return fmt.Errorf(
				"authenticating ServiceAccount token with configured audience %q: %w",
				configuredAud,
				err,
			)
		}
	}

	unconfiguredTok, unconfiguredClaims, err := mintAndVerifySAToken(
		ctx,
		clientset,
		keys,
		[]string{UnconfiguredServiceAccountAudience},
		600,
	)
	if err != nil {
		return fmt.Errorf("minting ServiceAccount token for unconfigured audience: %w", err)
	}
	if err := verifyTokenClaims(
		unconfiguredClaims,
		sai.Issuer,
		expectedSub,
		UnconfiguredServiceAccountAudience,
	); err != nil {
		return err
	}
	if err := ValidateTokenAuth(ctx, restCfg, unconfiguredTok, nil); err != nil {
		return fmt.Errorf(
			"verifying rejection of unconfigured audience %q: %w",
			UnconfiguredServiceAccountAudience,
			err,
		)
	}

	if sai.MaxTokenExpiration != nil {
		maxSec := int64(sai.MaxTokenExpiration.Seconds())
		requestedSec := max(maxSec*2, 1200)
		_, cappedClaims, err := mintAndVerifySAToken(
			ctx,
			clientset,
			keys,
			[]string{configuredAuds[0]},
			requestedSec,
		)
		if err != nil {
			return fmt.Errorf("minting ServiceAccount token to test maxTokenExpiration: %w", err)
		}
		iat, err := int64Claim(cappedClaims, "iat")
		if err != nil {
			return err
		}
		exp, err := int64Claim(cappedClaims, "exp")
		if err != nil {
			return err
		}
		lifetime := exp - iat
		if lifetime <= 0 || lifetime > maxSec+5 || lifetime >= requestedSec {
			return fmt.Errorf(
				"token lifetime = %ds (requested %ds), expected cap <= %ds from maxTokenExpiration",
				lifetime, requestedSec, maxSec,
			)
		}
	}

	return nil
}

func mintAndVerifySAToken(
	ctx context.Context,
	clientset kubernetes.Interface,
	keys map[string]crypto.PublicKey,
	audiences []string,
	expirationSeconds int64,
) (token string, claims map[string]any, err error) {
	req := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         audiences,
			ExpirationSeconds: &expirationSeconds,
		},
	}
	resp, err := clientset.CoreV1().ServiceAccounts(ServiceAccountIssuerE2ENamespace).
		CreateToken(ctx, ServiceAccountIssuerE2EName, req, metav1.CreateOptions{})
	if err != nil {
		return "", nil, err
	}
	if resp.Status.Token == "" {
		return "", nil, errors.New("TokenRequest returned empty token")
	}
	claims, err = VerifyServiceAccountJWT(resp.Status.Token, keys)
	if err != nil {
		return "", nil, fmt.Errorf("verifying JWT signature against /openid/v1/jwks: %w", err)
	}
	return resp.Status.Token, claims, nil
}

func verifyTokenClaims(claims map[string]any, wantIssuer, wantSub, wantAudience string) error {
	gotIss, _ := claims["iss"].(string)
	if gotIss != wantIssuer {
		return fmt.Errorf("token iss = %q, want %q", gotIss, wantIssuer)
	}
	gotSub, _ := claims["sub"].(string)
	if gotSub != wantSub {
		return fmt.Errorf("token sub = %q, want %q", gotSub, wantSub)
	}
	var gotAuds []string
	switch v := claims["aud"].(type) {
	case string:
		gotAuds = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				gotAuds = append(gotAuds, s)
			}
		}
	}
	if !slices.Contains(gotAuds, wantAudience) {
		return fmt.Errorf("token aud = %v, want to contain %q", gotAuds, wantAudience)
	}
	return nil
}

func int64Claim(claims map[string]any, key string) (int64, error) {
	v, ok := claims[key].(float64)
	if !ok {
		return 0, fmt.Errorf("claim %q missing or non-numeric (%T)", key, claims[key])
	}
	return int64(v), nil
}

func parseJWKPublicKey(entry *jwkEntry) (crypto.PublicKey, error) {
	switch entry.Kty {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(entry.N)
		if err != nil {
			return nil, fmt.Errorf("decoding RSA JWK modulus: %w", err)
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(entry.E)
		if err != nil {
			return nil, fmt.Errorf("decoding RSA JWK exponent: %w", err)
		}
		eInt := new(big.Int).SetBytes(eBytes)
		if !eInt.IsInt64() || eInt.Int64() <= 0 {
			return nil, fmt.Errorf("invalid RSA JWK exponent %q", entry.E)
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(eInt.Int64()),
		}, nil

	case "EC":
		var curve elliptic.Curve
		switch entry.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported EC JWK curve %q", entry.Crv)
		}
		xBytes, err := base64.RawURLEncoding.DecodeString(entry.X)
		if err != nil {
			return nil, fmt.Errorf("decoding EC JWK X coordinate: %w", err)
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(entry.Y)
		if err != nil {
			return nil, fmt.Errorf("decoding EC JWK Y coordinate: %w", err)
		}
		coordLen := (curve.Params().BitSize + 7) / 8
		if len(xBytes) > coordLen || len(yBytes) > coordLen {
			return nil, fmt.Errorf(
				"invalid EC JWK coordinate lengths (%d, %d) for %s",
				len(xBytes),
				len(yBytes),
				entry.Crv,
			)
		}
		uncompressed := make([]byte, 1+2*coordLen)
		uncompressed[0] = 0x04
		copy(uncompressed[1+coordLen-len(xBytes):1+coordLen], xBytes)
		copy(uncompressed[1+2*coordLen-len(yBytes):1+2*coordLen], yBytes)
		ecPub, err := ecdsa.ParseUncompressedPublicKey(curve, uncompressed)
		if err != nil {
			return nil, fmt.Errorf("parsing EC JWK public key: %w", err)
		}
		return ecPub, nil

	default:
		return nil, fmt.Errorf("unsupported JWK kty %q", entry.Kty)
	}
}

func verifyJWTSignature(
	alg string,
	pub crypto.PublicKey,
	signingInput, sigBytes []byte,
) error {
	switch alg {
	case "RS256":
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("alg RS256 requires RSA public key, got %T", pub)
		}
		digest := sha256.Sum256(signingInput)
		if err := rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, digest[:], sigBytes); err != nil {
			return fmt.Errorf("verifying RS256 signature: %w", err)
		}
		return nil

	case "ES256", "ES384", "ES512":
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			return fmt.Errorf("alg %s requires ECDSA public key, got %T", alg, pub)
		}
		var (
			wantCurve elliptic.Curve
			digest    []byte
			keySize   int
		)
		switch alg {
		case "ES256":
			wantCurve = elliptic.P256()
			sum := sha256.Sum256(signingInput)
			digest = sum[:]
			keySize = 32
		case "ES384":
			wantCurve = elliptic.P384()
			sum := sha512.Sum384(signingInput)
			digest = sum[:]
			keySize = 48
		case "ES512":
			wantCurve = elliptic.P521()
			sum := sha512.Sum512(signingInput)
			digest = sum[:]
			keySize = 66
		}
		if ecPub.Curve != wantCurve {
			return fmt.Errorf(
				"alg %s requires curve %s, got %s",
				alg,
				wantCurve.Params().Name,
				ecPub.Curve.Params().Name,
			)
		}
		if len(sigBytes) != 2*keySize {
			return fmt.Errorf("alg %s expected %d-byte signature, got %d", alg, 2*keySize, len(sigBytes))
		}
		r := new(big.Int).SetBytes(sigBytes[:keySize])
		s := new(big.Int).SetBytes(sigBytes[keySize:])
		if !ecdsa.Verify(ecPub, digest, r, s) {
			return fmt.Errorf("invalid %s signature", alg)
		}
		return nil

	default:
		return fmt.Errorf("unsupported JWT signing algorithm %q", alg)
	}
}
