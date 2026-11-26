// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/rest"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer"
)

func TestEnableServiceAccountIssuerInClusterTemplate(t *testing.T) {
	t.Parallel()

	bundle, err := NewBundle("10.128.0.12")
	require.NoError(t, err)
	sai := DefaultE2EServiceAccountIssuer()

	for _, rel := range []string{
		"examples/capi-quick-start/aws-cluster-cilium-helm-addon.yaml",
		"examples/capi-quick-start/docker-cluster-cilium-helm-addon.yaml",
		"examples/capi-quick-start/nutanix-cluster-cilium-helm-addon.yaml",
	} {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", rel))
			require.NoError(t, err)

			withAuth, err := EnableAuthenticationInClusterTemplate(raw, bundle.AuthenticationConfig())
			require.NoError(t, err)

			patched, err := EnableServiceAccountIssuerInClusterTemplate(withAuth, sai)
			require.NoError(t, err)
			assert.Contains(t, string(patched), `"${WORKER_MACHINE_COUNT}"`)

			docs := strings.Split(string(patched), "\n---\n")
			var clusterDoc string
			for _, d := range docs {
				if strings.Contains(d, "kind: Cluster\n") {
					clusterDoc = d
					break
				}
			}
			require.NotEmpty(t, clusterDoc)
			obj := map[string]any{}
			require.NoError(t, sigsyaml.Unmarshal([]byte(clusterDoc), &obj))
			vars, found, err := unstructured.NestedSlice(obj, "spec", "topology", "variables")
			require.NoError(t, err)
			require.True(t, found)

			foundSAI := false
			for _, item := range vars {
				m, ok := item.(map[string]any)
				if !ok || m["name"] != v1alpha1.ClusterConfigVariableName {
					continue
				}
				// Verify authentication and addons were preserved alongside serviceAccountIssuer.
				_, hasAuth, err := unstructured.NestedSlice(m, "value", authentication.VariableName, "jwt")
				require.NoError(t, err)
				assert.True(t, hasAuth, "expected existing authentication config to be preserved")

				_, hasAddons, err := unstructured.NestedMap(m, "value", "addons")
				require.NoError(t, err)
				assert.True(t, hasAddons, "expected existing addons config to be preserved")

				gotIssuer, ok, err := unstructured.NestedString(
					m, "value", serviceaccountissuer.VariableName, "issuer",
				)
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, DefaultServiceAccountIssuerURL, gotIssuer)

				gotJWKS, ok, err := unstructured.NestedString(
					m, "value", serviceaccountissuer.VariableName, "jwksURI",
				)
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, DefaultServiceAccountJWKSURI, gotJWKS)

				gotMaxExp, ok, err := unstructured.NestedString(
					m, "value", serviceaccountissuer.VariableName, "maxTokenExpiration",
				)
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, "2h0m0s", gotMaxExp)

				gotAuds, ok, err := unstructured.NestedStringSlice(
					m, "value", serviceaccountissuer.VariableName, "audiences",
				)
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, sai.Audiences, gotAuds)
				foundSAI = true
			}
			assert.True(t, foundSAI, "expected clusterConfig.serviceAccountIssuer to be injected")
		})
	}

	t.Run("creates clusterConfig variable when absent", func(t *testing.T) {
		t.Parallel()
		const minimalCluster = `apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: test
spec:
  topology:
    classRef:
      name: quick-start
`
		patched, err := EnableServiceAccountIssuerInClusterTemplate([]byte(minimalCluster), sai)
		require.NoError(t, err)

		obj := map[string]any{}
		require.NoError(t, sigsyaml.Unmarshal(patched, &obj))
		vars, found, err := unstructured.NestedSlice(obj, "spec", "topology", "variables")
		require.NoError(t, err)
		require.True(t, found)
		require.Len(t, vars, 1)
		m, ok := vars[0].(map[string]any)
		require.True(t, ok)
		gotIssuer, ok, err := unstructured.NestedString(m, "value", serviceaccountissuer.VariableName, "issuer")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, DefaultServiceAccountIssuerURL, gotIssuer)
	})
}

func TestServiceAccountIssuerControlPlanePredicates(t *testing.T) {
	t.Parallel()

	sai := DefaultE2EServiceAccountIssuer()
	wantArgs := ExpectedServiceAccountIssuerExtraArgs(sai)
	assert.Equal(t, map[string]string{
		serviceaccountissuer.ServiceAccountIssuerArg:      DefaultServiceAccountIssuerURL,
		serviceaccountissuer.APIAudiencesArg:              DefaultKubernetesServiceAccountAudience + "," + DefaultServiceAccountAudience,
		serviceaccountissuer.ServiceAccountJWKSURIArg:     DefaultServiceAccountJWKSURI,
		serviceaccountissuer.ServiceAccountMaxTokenExpArg: "2h0m0s",
	}, wantArgs)

	minimal := v1alpha1.ServiceAccountIssuer{Issuer: "https://oidc.example.com"}
	assert.Equal(t, map[string]string{
		serviceaccountissuer.ServiceAccountIssuerArg: "https://oidc.example.com",
	}, ExpectedServiceAccountIssuerExtraArgs(minimal))

	three := int32(3)
	one := int32(1)
	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: &three,
			KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
				ClusterConfiguration: bootstrapv1.ClusterConfiguration{
					APIServer: bootstrapv1.APIServer{
						ExtraArgs: []bootstrapv1.Arg{
							{
								Name: serviceaccountissuer.APIAudiencesArg,
								Value: new(
									DefaultKubernetesServiceAccountAudience + "," + DefaultServiceAccountAudience,
								),
							},
							{
								Name:  authentication.AuthenticationConfigArg,
								Value: new(authentication.AuthenticationConfigPath),
							},
							{
								Name:  serviceaccountissuer.ServiceAccountIssuerArg,
								Value: new(DefaultServiceAccountIssuerURL),
							},
							{
								Name:  serviceaccountissuer.ServiceAccountJWKSURIArg,
								Value: new(DefaultServiceAccountJWKSURI),
							},
							{
								Name:  serviceaccountissuer.ServiceAccountMaxTokenExpArg,
								Value: new("2h0m0s"),
							},
						},
					},
				},
			},
		},
		Status: controlplanev1.KubeadmControlPlaneStatus{
			ObservedGeneration: 2,
			Replicas:           &three,
			ReadyReplicas:      &three,
			UpToDateReplicas:   &three,
		},
	}

	assert.True(t, KubeadmControlPlaneHasAPIServerArg(
		kcp, serviceaccountissuer.ServiceAccountIssuerArg, DefaultServiceAccountIssuerURL,
	))
	assert.False(t, KubeadmControlPlaneHasAPIServerArg(
		kcp, serviceaccountissuer.ServiceAccountIssuerArg, "https://other.example.com",
	))
	assert.True(t, ServiceAccountIssuerConfigCurrent(kcp, sai))

	staleGen := kcp.DeepCopy()
	staleGen.Status.ObservedGeneration = 1
	assert.False(t, ServiceAccountIssuerConfigCurrent(staleGen, sai))

	laggingReplica := kcp.DeepCopy()
	laggingReplica.Status.UpToDateReplicas = &one
	assert.False(t, ServiceAccountIssuerConfigCurrent(laggingReplica, sai))

	wrongArg := kcp.DeepCopy()
	wrongArg.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs[0].Value = new("wrong-aud")
	assert.False(t, ServiceAccountIssuerConfigCurrent(wrongArg, sai))

	nilArg := kcp.DeepCopy()
	nilArg.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs[0].Value = nil
	assert.False(t, ServiceAccountIssuerConfigCurrent(nilArg, sai))
}

func TestVerifyOIDCDiscoveryAndJWKS(t *testing.T) {
	t.Parallel()

	sai := DefaultE2EServiceAccountIssuer()
	goodDoc := &OIDCDiscoveryDocument{
		Issuer:                           DefaultServiceAccountIssuerURL,
		JWKSURI:                          DefaultServiceAccountJWKSURI,
		ResponseTypesSupported:           []string{"id_token"},
		IDTokenSigningAlgValuesSupported: []string{"RS256"},
	}
	require.NoError(t, VerifyOIDCDiscovery(goodDoc, sai))

	wrongIssuer := *goodDoc
	wrongIssuer.Issuer = "https://kubernetes.default.svc.cluster.local"
	require.ErrorContains(t, VerifyOIDCDiscovery(&wrongIssuer, sai), "issuer")

	wrongJWKS := *goodDoc
	wrongJWKS.JWKSURI = "https://10.128.0.1/openid/v1/jwks"
	require.ErrorContains(t, VerifyOIDCDiscovery(&wrongJWKS, sai), "jwks_uri")

	noAlgs := *goodDoc
	noAlgs.IDTokenSigningAlgValuesSupported = nil
	require.ErrorContains(t, VerifyOIDCDiscovery(&noAlgs, sai), "id_token_signing_alg_values_supported")

	t.Run("RS256 signature verification", func(t *testing.T) {
		t.Parallel()
		bundle, err := NewBundle("10.128.0.12")
		require.NoError(t, err)
		token, err := bundle.MintJWT(map[string]any{
			"iss": DefaultServiceAccountIssuerURL,
			"sub": "system:serviceaccount:default:test",
			"aud": []string{DefaultServiceAccountAudience},
		})
		require.NoError(t, err)

		keys := map[string]crypto.PublicKey{
			jwksKeyID: &bundle.PrivateKey.PublicKey,
		}
		claims, err := VerifyServiceAccountJWT(token, keys)
		require.NoError(t, err)
		assert.Equal(t, DefaultServiceAccountIssuerURL, claims["iss"])

		otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		_, err = VerifyServiceAccountJWT(token, map[string]crypto.PublicKey{
			jwksKeyID: &otherKey.PublicKey,
		})
		require.Error(t, err)

		_, err = VerifyServiceAccountJWT(token, map[string]crypto.PublicKey{
			"other-kid": &bundle.PrivateKey.PublicKey,
		})
		require.ErrorContains(t, err, "kid")
	})

	t.Run("ES256 signature verification", func(t *testing.T) {
		t.Parallel()
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		headerJSON, err := json.Marshal(map[string]string{
			"alg": "ES256",
			"kid": "ec-1",
			"typ": "JWT",
		})
		require.NoError(t, err)
		payloadJSON, err := json.Marshal(map[string]any{
			"iss": DefaultServiceAccountIssuerURL,
			"sub": "system:serviceaccount:default:ec",
			"aud": []string{DefaultServiceAccountAudience},
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		require.NoError(t, err)

		signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
			base64.RawURLEncoding.EncodeToString(payloadJSON)
		digest := sha256.Sum256([]byte(signingInput))
		r, s, err := ecdsa.Sign(rand.Reader, ecKey, digest[:])
		require.NoError(t, err)
		sig := make([]byte, 64)
		r.FillBytes(sig[:32])
		s.FillBytes(sig[32:])
		token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

		pubBytes, err := ecKey.PublicKey.Bytes()
		require.NoError(t, err)
		require.Equal(t, byte(0x04), pubBytes[0])
		parsedPub, err := parseJWKPublicKey(&jwkEntry{
			Kty: "EC",
			Crv: "P-256",
			Kid: "ec-1",
			X:   base64.RawURLEncoding.EncodeToString(pubBytes[1:33]),
			Y:   base64.RawURLEncoding.EncodeToString(pubBytes[33:65]),
		})
		require.NoError(t, err)

		claims, err := VerifyServiceAccountJWT(token, map[string]crypto.PublicKey{
			"ec-1": parsedPub,
		})
		require.NoError(t, err)
		assert.Equal(t, "system:serviceaccount:default:ec", claims["sub"])

		// Sign SHA-384 with the P-256 key, zero-padded to 96 bytes, claiming alg=ES384.
		// VerifyServiceAccountJWT must reject the curve/alg mismatch (ES384 requires P-384).
		mismatchHeader, err := json.Marshal(map[string]string{
			"alg": "ES384",
			"kid": "ec-1",
			"typ": "JWT",
		})
		require.NoError(t, err)
		mismatchInput := base64.RawURLEncoding.EncodeToString(mismatchHeader) + "." +
			base64.RawURLEncoding.EncodeToString(payloadJSON)
		digest384 := sha512.Sum384([]byte(mismatchInput))
		r384, s384, err := ecdsa.Sign(rand.Reader, ecKey, digest384[:])
		require.NoError(t, err)
		paddedSig := make([]byte, 96)
		r384.FillBytes(paddedSig[:48])
		s384.FillBytes(paddedSig[48:])
		mismatchedToken := mismatchInput + "." + base64.RawURLEncoding.EncodeToString(paddedSig)

		_, err = VerifyServiceAccountJWT(mismatchedToken, map[string]crypto.PublicKey{
			"ec-1": parsedPub,
		})
		require.ErrorContains(t, err, "curve")
	})
}

type fakeAPIServerOptions struct {
	issuer        string
	jwksURI       string
	apiAudiences  []string
	maxExpiration time.Duration
	disableCap    bool
	acceptAnyAud  bool
}

func newFakeServiceAccountAPIServer(t *testing.T, opts fakeAPIServerOptions) *httptest.Server {
	t.Helper()

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwksBody, err := buildJWKS(signingKey)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(OIDCDiscoveryDocument{
				Issuer:                           opts.issuer,
				JWKSURI:                          opts.jwksURI,
				ResponseTypesSupported:           []string{"id_token"},
				IDTokenSigningAlgValuesSupported: []string{"RS256"},
			})

		case r.Method == http.MethodGet && r.URL.Path == "/openid/v1/jwks":
			_, _ = w.Write([]byte(jwksBody))

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/namespaces":
			var ns corev1.Namespace
			_ = json.NewDecoder(r.Body).Decode(&ns)
			_ = json.NewEncoder(w).Encode(&ns)

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v1/namespaces/"):
			_ = json.NewEncoder(w).Encode(&metav1.Status{Status: metav1.StatusSuccess})

		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/namespaces/"+ServiceAccountIssuerE2ENamespace+"/serviceaccounts":
			var sa corev1.ServiceAccount
			_ = json.NewDecoder(r.Body).Decode(&sa)
			_ = json.NewEncoder(w).Encode(&sa)

		case r.Method == http.MethodPost &&
			r.URL.Path == "/api/v1/namespaces/"+ServiceAccountIssuerE2ENamespace+
				"/serviceaccounts/"+ServiceAccountIssuerE2EName+"/token":
			var tr authenticationv1.TokenRequest
			if err := json.NewDecoder(r.Body).Decode(&tr); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			expSec := int64(3600)
			if tr.Spec.ExpirationSeconds != nil {
				expSec = *tr.Spec.ExpirationSeconds
			}
			if !opts.disableCap && opts.maxExpiration > 0 && expSec > int64(opts.maxExpiration.Seconds()) {
				expSec = int64(opts.maxExpiration.Seconds())
			}
			now := time.Now().Truncate(time.Second)
			exp := now.Add(time.Duration(expSec) * time.Second)
			auds := tr.Spec.Audiences
			if len(auds) == 0 {
				auds = opts.apiAudiences
			}
			sub := serviceaccount.MakeUsername(ServiceAccountIssuerE2ENamespace, ServiceAccountIssuerE2EName)
			bundle := &Bundle{IssuerURL: opts.issuer, PrivateKey: signingKey}
			tok, err := bundle.MintJWT(map[string]any{
				"iss": opts.issuer,
				"sub": sub,
				"aud": auds,
				"iat": now.Unix(),
				"exp": exp.Unix(),
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			tr.Status = authenticationv1.TokenRequestStatus{
				Token:               tok,
				ExpirationTimestamp: metav1.NewTime(exp),
			}
			_ = json.NewEncoder(w).Encode(&tr)

		case r.Method == http.MethodPost && r.URL.Path == "/apis/authentication.k8s.io/v1/selfsubjectreviews":
			bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			claims, err := VerifyServiceAccountJWT(bearer, map[string]crypto.PublicKey{
				jwksKeyID: &signingKey.PublicKey,
			})
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(apierrors.NewUnauthorized(err.Error()).ErrStatus)
				return
			}
			var tokenAuds []string
			switch v := claims["aud"].(type) {
			case string:
				tokenAuds = []string{v}
			case []any:
				for _, item := range v {
					if s, ok := item.(string); ok {
						tokenAuds = append(tokenAuds, s)
					}
				}
			}
			matched := opts.acceptAnyAud
			for _, a := range tokenAuds {
				if slices.Contains(opts.apiAudiences, a) {
					matched = true
					break
				}
			}
			if !matched {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(apierrors.NewUnauthorized("unrecognized audience").ErrStatus)
				return
			}
			sub, _ := claims["sub"].(string)
			_ = json.NewEncoder(w).Encode(&authenticationv1.SelfSubjectReview{
				TypeMeta: metav1.TypeMeta{
					APIVersion: authenticationv1.SchemeGroupVersion.String(),
					Kind:       "SelfSubjectReview",
				},
				Status: authenticationv1.SelfSubjectReviewStatus{
					UserInfo: authenticationv1.UserInfo{
						Username: sub,
						Groups: append(
							serviceaccount.MakeGroupNames(ServiceAccountIssuerE2ENamespace),
							"system:authenticated",
						),
					},
				},
			})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fakeRESTConfig(url string) *rest.Config {
	return &rest.Config{
		Host: url,
		ContentConfig: rest.ContentConfig{
			ContentType: "application/json",
		},
	}
}

func TestVerifyWorkloadServiceAccountIssuer(t *testing.T) {
	t.Parallel()

	sai := DefaultE2EServiceAccountIssuer()

	t.Run("succeeds when discovery, JWKS signature, audiences, and maxTokenExpiration all match", func(t *testing.T) {
		t.Parallel()
		srv := newFakeServiceAccountAPIServer(t, fakeAPIServerOptions{
			issuer:        DefaultServiceAccountIssuerURL,
			jwksURI:       DefaultServiceAccountJWKSURI,
			apiAudiences:  sai.Audiences,
			maxExpiration: DefaultServiceAccountMaxTokenExpiration,
		})
		require.NoError(t, VerifyWorkloadServiceAccountIssuer(t.Context(), fakeRESTConfig(srv.URL), sai))
	})

	t.Run("fails when discovery issuer mismatches", func(t *testing.T) {
		t.Parallel()
		srv := newFakeServiceAccountAPIServer(t, fakeAPIServerOptions{
			issuer:        "https://wrong-issuer.example.com",
			jwksURI:       DefaultServiceAccountJWKSURI,
			apiAudiences:  sai.Audiences,
			maxExpiration: DefaultServiceAccountMaxTokenExpiration,
		})
		require.ErrorContains(
			t,
			VerifyWorkloadServiceAccountIssuer(t.Context(), fakeRESTConfig(srv.URL), sai),
			"issuer",
		)
	})

	t.Run("fails when second configured audience is rejected by API server", func(t *testing.T) {
		t.Parallel()
		srv := newFakeServiceAccountAPIServer(t, fakeAPIServerOptions{
			issuer:        DefaultServiceAccountIssuerURL,
			jwksURI:       DefaultServiceAccountJWKSURI,
			apiAudiences:  []string{sai.Audiences[0]},
			maxExpiration: DefaultServiceAccountMaxTokenExpiration,
		})
		require.ErrorContains(
			t,
			VerifyWorkloadServiceAccountIssuer(t.Context(), fakeRESTConfig(srv.URL), sai),
			sai.Audiences[1],
		)
	})

	t.Run("fails when unconfigured audience is accepted by API server", func(t *testing.T) {
		t.Parallel()
		srv := newFakeServiceAccountAPIServer(t, fakeAPIServerOptions{
			issuer:        DefaultServiceAccountIssuerURL,
			jwksURI:       DefaultServiceAccountJWKSURI,
			apiAudiences:  sai.Audiences,
			maxExpiration: DefaultServiceAccountMaxTokenExpiration,
			acceptAnyAud:  true,
		})
		require.ErrorContains(
			t,
			VerifyWorkloadServiceAccountIssuer(t.Context(), fakeRESTConfig(srv.URL), sai),
			"expected token to be rejected",
		)
	})

	t.Run("fails when maxTokenExpiration does not cap token lifetime", func(t *testing.T) {
		t.Parallel()
		srv := newFakeServiceAccountAPIServer(t, fakeAPIServerOptions{
			issuer:        DefaultServiceAccountIssuerURL,
			jwksURI:       DefaultServiceAccountJWKSURI,
			apiAudiences:  sai.Audiences,
			maxExpiration: DefaultServiceAccountMaxTokenExpiration,
			disableCap:    true,
		})
		require.ErrorContains(
			t,
			VerifyWorkloadServiceAccountIssuer(t.Context(), fakeRESTConfig(srv.URL), sai),
			"maxTokenExpiration",
		)
	})
}

func TestVerifyWorkloadServiceAccountIssuer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real kube-apiserver envtest integration test in short mode")
	}
	t.Parallel()

	keyDir := t.TempDir()
	saKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPath := filepath.Join(keyDir, "sa.key")
	pubDER, err := x509.MarshalPKIXPublicKey(&saKey.PublicKey)
	require.NoError(t, err)
	pubPath := filepath.Join(keyDir, "sa.pub")

	require.NoError(t, os.WriteFile(
		keyPath,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(saKey)}),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		pubPath,
		pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
		0o600,
	))

	sai := DefaultE2EServiceAccountIssuer()
	expectedArgs := ExpectedServiceAccountIssuerExtraArgs(sai)

	testEnv := &envtest.Environment{}
	apiArgs := testEnv.ControlPlane.GetAPIServer().Configure()
	apiArgs.Set(
		serviceaccountissuer.ServiceAccountIssuerArg,
		expectedArgs[serviceaccountissuer.ServiceAccountIssuerArg],
	)
	apiArgs.Set(
		serviceaccountissuer.APIAudiencesArg,
		expectedArgs[serviceaccountissuer.APIAudiencesArg],
	)
	apiArgs.Set(
		serviceaccountissuer.ServiceAccountJWKSURIArg,
		expectedArgs[serviceaccountissuer.ServiceAccountJWKSURIArg],
	)
	apiArgs.Set(
		serviceaccountissuer.ServiceAccountMaxTokenExpArg,
		expectedArgs[serviceaccountissuer.ServiceAccountMaxTokenExpArg],
	)
	apiArgs.Set("service-account-signing-key-file", keyPath)
	apiArgs.Set("service-account-key-file", pubPath)

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, testEnv.Stop())
	})

	require.NoError(t, VerifyWorkloadServiceAccountIssuer(t.Context(), cfg, sai))
	// Second invocation verifies idempotency when retried inside Ginkgo Eventually.
	require.NoError(t, VerifyWorkloadServiceAccountIssuer(t.Context(), cfg, sai))
}
