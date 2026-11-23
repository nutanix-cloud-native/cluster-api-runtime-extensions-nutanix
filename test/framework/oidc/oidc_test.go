// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oidc

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
)

func TestStaticServiceIPFromCIDR(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cidr    string
		want    string
		wantErr bool
	}{
		{
			name: "caren default service CIDR",
			cidr: "10.128.0.0/12",
			want: "10.128.0.12",
		},
		{
			name: "kubeadm default service CIDR",
			cidr: "10.96.0.0/12",
			want: "10.96.0.12",
		},
		{
			name: "slash 28 subnet",
			cidr: "172.20.0.16/28",
			want: "172.20.0.28",
		},
		{
			name: "ipv6 service CIDR",
			cidr: "fd00:100:64::/112",
			want: "fd00:100:64::c",
		},
		{
			name:    "subnet too small for static offset",
			cidr:    "10.0.0.0/29",
			wantErr: true,
		},
		{
			name:    "invalid CIDR",
			cidr:    "not-a-cidr",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := StaticServiceIPFromCIDR(tt.cidr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveClusterTemplateServiceCIDR(t *testing.T) {
	t.Parallel()

	const nutanixStyleMultiDoc = `apiVersion: v1
kind: Secret
metadata:
  name: creds
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: ${CLUSTER_NAME}
spec:
  clusterNetwork:
    services:
      cidrBlocks:
      - ${SERVICE_CIDR:-10.128.0.0/12}
`

	t.Run("uses lookupVar override when set", func(t *testing.T) {
		t.Parallel()
		cidr, err := ResolveClusterTemplateServiceCIDR([]byte(nutanixStyleMultiDoc), func(k string) (string, bool) {
			if k == "SERVICE_CIDR" {
				return "172.30.0.0/16", true
			}
			return "", false
		})
		require.NoError(t, err)
		assert.Equal(t, "172.30.0.0/16", cidr)
	})

	t.Run("uses inline default when lookupVar is unset", func(t *testing.T) {
		t.Parallel()
		cidr, err := ResolveClusterTemplateServiceCIDR([]byte(nutanixStyleMultiDoc), func(string) (string, bool) {
			return "", false
		})
		require.NoError(t, err)
		assert.Equal(t, "10.128.0.0/12", cidr)
	})

	t.Run("falls back to kubeadm default when services.cidrBlocks omitted", func(t *testing.T) {
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
		cidr, err := ResolveClusterTemplateServiceCIDR([]byte(minimalCluster), nil)
		require.NoError(t, err)
		assert.Equal(t, "10.96.0.0/12", cidr)
	})
}

func TestVerifyServiceIPInCluster(t *testing.T) {
	t.Parallel()

	cluster := &clusterv1.Cluster{
		Spec: clusterv1.ClusterSpec{
			ClusterNetwork: clusterv1.ClusterNetwork{
				Services: clusterv1.NetworkRanges{
					CIDRBlocks: []string{"10.128.0.0/12"},
				},
			},
		},
	}
	require.NoError(t, VerifyServiceIPInCluster("10.128.0.12", cluster))
	require.Error(t, VerifyServiceIPInCluster("10.96.0.12", cluster))
	require.Error(t, VerifyServiceIPInCluster("10.128.0.99", cluster))
}

func TestBundleCertificateSANsAndStdlibJWT(t *testing.T) {
	t.Parallel()

	t.Run("ipv4 clusterIP puts IP in IPAddresses only and never in DNSNames", func(t *testing.T) {
		t.Parallel()
		bundle, err := NewBundle("10.128.0.12")
		require.NoError(t, err)
		assert.Equal(t, "https://10.128.0.12", bundle.IssuerURL)
		assert.Equal(t, "https://10.128.0.12/.well-known/openid-configuration", bundle.DiscoveryURL)

		block, _ := pem.Decode(bundle.ServerCertPEM)
		require.NotNil(t, block)
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		for _, dns := range cert.DNSNames {
			assert.Nilf(t, net.ParseIP(dns), "DNSNames must not contain IP literal %q", dns)
		}
		assert.Contains(t, cert.DNSNames, "mock-oidc.oidc-mock.svc")

		var gotIPs []string
		for _, ip := range cert.IPAddresses {
			gotIPs = append(gotIPs, ip.String())
		}
		assert.Contains(t, gotIPs, "10.128.0.12")
		assert.Contains(t, gotIPs, "127.0.0.1")

		token, err := bundle.MintJWT(map[string]any{
			"sub":    "123",
			"email":  "test@example.com",
			"groups": []string{"e2e-group"},
			"aud":    "e2e-aud",
		})
		require.NoError(t, err)

		parts := strings.Split(token, ".")
		require.Len(t, parts, 3)

		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		require.NoError(t, err)
		var header map[string]string
		require.NoError(t, json.Unmarshal(headerBytes, &header))
		assert.Equal(t, "RS256", header["alg"])
		assert.Equal(t, "e2e", header["kid"])

		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(payloadBytes, &payload))
		assert.Equal(t, "https://10.128.0.12", payload["iss"])
		assert.Equal(t, "test@example.com", payload["email"])

		sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
		require.NoError(t, err)
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		require.NoError(t, rsa.VerifyPKCS1v15(&bundle.PrivateKey.PublicKey, crypto.SHA256, digest[:], sigBytes))

		authCfg := bundle.AuthenticationConfig()
		require.Len(t, authCfg.JWT, 1)
		assert.Equal(t, bundle.IssuerURL, authCfg.JWT[0].Issuer.URL)
		assert.Equal(t, bundle.CAPEM, authCfg.JWT[0].Issuer.CertificateAuthority)
	})

	t.Run("ipv6 clusterIP brackets URL host", func(t *testing.T) {
		t.Parallel()
		bundle, err := NewBundle("fd00:100:64::c")
		require.NoError(t, err)
		assert.Equal(t, "https://[fd00:100:64::c]", bundle.IssuerURL)
		assert.Equal(t, "https://[fd00:100:64::c]/.well-known/openid-configuration", bundle.DiscoveryURL)
	})
}

func TestEnableAuthenticationInClusterTemplate(t *testing.T) {
	t.Parallel()

	bundle, err := NewBundle("10.128.0.12")
	require.NoError(t, err)

	for _, rel := range []string{
		"examples/capi-quick-start/aws-cluster-cilium-helm-addon.yaml",
		"examples/capi-quick-start/docker-cluster-cilium-helm-addon.yaml",
		"examples/capi-quick-start/nutanix-cluster-cilium-helm-addon.yaml",
	} {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", rel))
			require.NoError(t, err)

			cidr, err := ResolveClusterTemplateServiceCIDR(raw, nil)
			require.NoError(t, err)
			assert.Equal(t, "10.128.0.0/12", cidr)

			patched, err := EnableAuthenticationInClusterTemplate(raw, bundle.AuthenticationConfig())
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

			foundAuth := false
			for _, item := range vars {
				m, ok := item.(map[string]any)
				if !ok || m["name"] != "clusterConfig" {
					continue
				}
				jwtSlice, ok, err := unstructured.NestedSlice(m, "value", "authentication", "jwt")
				require.NoError(t, err)
				require.True(t, ok)
				require.Len(t, jwtSlice, 1)
				firstJWT, ok := jwtSlice[0].(map[string]any)
				require.True(t, ok)
				url, ok, err := unstructured.NestedString(firstJWT, "issuer", "url")
				require.NoError(t, err)
				require.True(t, ok)
				assert.Equal(t, "https://10.128.0.12", url)
				foundAuth = true
			}
			assert.True(t, foundAuth, "expected clusterConfig.authentication to be injected")
		})
	}
}

func TestClusterAndControlPlanePredicates(t *testing.T) {
	t.Parallel()

	settled := &clusterv1.Cluster{
		Status: clusterv1.ClusterStatus{
			Conditions: []metav1.Condition{
				{Type: clusterv1.ClusterAvailableCondition, Status: metav1.ConditionTrue},
				{Type: clusterv1.ClusterControlPlaneMachinesUpToDateCondition, Status: metav1.ConditionTrue},
				{Type: clusterv1.ClusterRollingOutCondition, Status: metav1.ConditionFalse},
			},
		},
	}
	assert.True(t, ClusterControlPlaneSettled(settled))

	rolling := settled.DeepCopy()
	rolling.Status.Conditions[2].Status = metav1.ConditionTrue
	assert.False(t, ClusterControlPlaneSettled(rolling))

	readyOnly := &clusterv1.Cluster{
		Status: clusterv1.ClusterStatus{
			Conditions: []metav1.Condition{
				{Type: clusterv1.ReadyCondition, Status: metav1.ConditionTrue},
			},
		},
	}
	assert.False(t, ClusterControlPlaneSettled(readyOnly))

	three := int32(3)
	one := int32(1)
	kcpHA := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: &three,
			KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
				Files: []bootstrapv1.File{{Path: authentication.AuthenticationConfigPath}},
			},
		},
		Status: controlplanev1.KubeadmControlPlaneStatus{
			ObservedGeneration: 2,
			Replicas:           &three,
			ReadyReplicas:      &three,
			UpToDateReplicas:   &three,
		},
	}
	assert.True(t, KubeadmControlPlaneAtReplicas(kcpHA, 3))
	assert.False(t, KubeadmControlPlaneAtReplicas(kcpHA, 1))
	assert.True(t, AuthenticationConfigCurrent(kcpHA))

	lagging := kcpHA.DeepCopy()
	lagging.Status.ReadyReplicas = &one
	assert.False(t, AuthenticationConfigCurrent(lagging))

	noFile := kcpHA.DeepCopy()
	noFile.Spec.KubeadmConfigSpec.Files = nil
	assert.False(t, AuthenticationConfigCurrent(noFile))
}

func TestBearerOnlyConfigDropsAdminCredentials(t *testing.T) {
	t.Parallel()

	cfg := &rest.Config{
		Host:            "https://apiserver",
		Username:        "kubernetes-admin",
		Password:        "secret",
		BearerToken:     "admin-token",
		BearerTokenFile: "/tmp/token",
		TLSClientConfig: rest.TLSClientConfig{
			CertFile: "/tmp/admin.crt",
			KeyFile:  "/tmp/admin.key",
			CertData: []byte("cert"),
			KeyData:  []byte("key"),
		},
	}
	out := BearerOnlyConfig(cfg, "jwt")
	assert.Equal(t, "jwt", out.BearerToken)
	assert.Empty(t, out.BearerTokenFile)
	assert.Empty(t, out.CertFile)
	assert.Empty(t, out.KeyFile)
	assert.Nil(t, out.CertData)
	assert.Nil(t, out.KeyData)
	assert.Empty(t, out.Username)
	assert.Empty(t, out.Password)
	assert.Equal(t, "kubernetes-admin", cfg.Username)
	assert.NotNil(t, cfg.CertData)
}

func TestCheckAndValidateTokenAuth(t *testing.T) {
	t.Parallel()

	want := &ExpectedUserInfo{
		Username: "test@example.com",
		Groups:   []string{"e2e-group"},
		Extra: map[string][]string{
			"example.org/foo": {"bar"},
		},
	}

	goodInfo := authenticationv1.UserInfo{
		Username: "test@example.com",
		Groups:   []string{"e2e-group", "system:authenticated"},
		Extra: map[string]authenticationv1.ExtraValue{
			"example.org/foo": {"bar"},
		},
	}
	require.NoError(t, CheckUserInfo(goodInfo, want))

	wrongGroup := goodInfo.DeepCopy()
	wrongGroup.Groups = []string{"other-group", "system:authenticated"}
	require.ErrorContains(t, CheckUserInfo(*wrongGroup, want), "groups")

	wrongExtra := goodInfo.DeepCopy()
	wrongExtra.Extra = map[string]authenticationv1.ExtraValue{
		"example.org/foo": {"wrong"},
	}
	require.ErrorContains(t, CheckUserInfo(*wrongExtra, want), "example.org/foo")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("Authorization") {
		case "Bearer valid-token":
			resp := authenticationv1.SelfSubjectReview{
				TypeMeta: metav1.TypeMeta{
					APIVersion: authenticationv1.SchemeGroupVersion.String(),
					Kind:       "SelfSubjectReview",
				},
				Status: authenticationv1.SelfSubjectReviewStatus{UserInfo: goodInfo},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			status := apierrors.NewUnauthorized("invalid bearer token").ErrStatus
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(status)
		}
	}))
	t.Cleanup(srv.Close)

	restCfg := &rest.Config{Host: srv.URL}
	require.NoError(t, ValidateTokenAuth(t.Context(), restCfg, "valid-token", want))
	require.NoError(t, ValidateTokenAuth(t.Context(), restCfg, "bad-token", nil))
	require.Error(t, ValidateTokenAuth(t.Context(), restCfg, "valid-token", nil))
	require.Error(t, ValidateTokenAuth(t.Context(), restCfg, "bad-token", want))
	_ = schema.GroupResource{}
}
