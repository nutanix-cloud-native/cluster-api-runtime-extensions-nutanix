// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

func Test_generateCredentialsSecretFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		configs     []providerConfig
		clusterName string
		wantFile    *cabpkv1.File
	}{
		{
			name:        "empty configs, expect no file",
			configs:     nil,
			clusterName: "test-cluster",
			wantFile:    nil,
		},
		{
			name: "config with no static credentials, expect no file",
			configs: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			clusterName: "test-cluster",
			wantFile:    nil,
		},
		{
			name: "config with static credentials, expect a file",
			configs: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
				},
			},
			clusterName: "test-cluster",
			wantFile: &cabpkv1.File{
				Path:        "/etc/kubernetes/static-image-credentials.json",
				Permissions: "0600",
				ContentFrom: &cabpkv1.FileSource{
					Secret: cabpkv1.SecretFileSource{
						Name: "test-cluster-static-credential-provider-response",
						Key:  "static-credential-provider",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotFile := generateCredentialsSecretFile(tt.configs, tt.clusterName)
			assert.Equal(t, tt.wantFile, gotFile)
		})
	}
}

func Test_generateCredentialsSecret(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		configs     []providerConfig
		clusterName string
		namespace   string
		wantSecret  *corev1.Secret
	}{
		{
			name:        "empty configs, expect no Secret",
			configs:     nil,
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			wantSecret:  nil,
		},
		{
			name: "config with no static credentials, expect no Secret",
			configs: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			wantSecret:  nil,
		},
		{
			name: "config with static credentials, expect a Secret",
			configs: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
				},
			},
			clusterName: "test-cluster",
			namespace:   "test-namespace",
			wantSecret: &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-static-credential-provider-response",
					Namespace: "test-namespace",
					Labels: map[string]string{
						"cluster.x-k8s.io/cluster-name":    "test-cluster",
						"clusterctl.cluster.x-k8s.io/move": "",
					},
				},
				StringData: map[string]string{
					"static-credential-provider": `{
  "kind":"CredentialProviderResponse",
  "apiVersion":"credentialprovider.kubelet.k8s.io/v1",
  "cacheKeyType":"Image",
  "cacheDuration":"0s",
  "auth":{
    "myregistry.com": {"username": "myuser", "password": "mypassword"}
  }
}`,
				},
				Type: corev1.SecretTypeOpaque,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotSecret, err := generateCredentialsSecret(tt.configs, tt.clusterName, tt.namespace)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSecret, gotSecret)
		})
	}
}

func Test_kubeletStaticCredentialProviderSecretContents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		configs      []providerConfig
		wantContents string
	}{
		{
			name: "config with no static credentials, expect empty string",
			configs: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			wantContents: "",
		},
		{
			name: "config with 'registry-1.docker.io', expect it to also add 'docker.io'",
			configs: []providerConfig{
				{
					URL:      "https://registry-1.docker.io",
					Username: "myuser",
					Password: "mypassword",
				},
			},
			wantContents: `{
  "kind":"CredentialProviderResponse",
  "apiVersion":"credentialprovider.kubelet.k8s.io/v1",
  "cacheKeyType":"Image",
  "cacheDuration":"0s",
  "auth":{
    "registry-1.docker.io": {"username": "myuser", "password": "mypassword"},
    "docker.io": {"username": "myuser", "password": "mypassword"}
  }
}`,
		},
		{
			name: "multiple configs with some static credentials, expect a string with multiple entries",
			configs: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
				},
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
				{
					URL:      "https://registry-1.docker.io",
					Username: "myuser",
					Password: "mypassword",
				},
				{
					URL:      "https://anotherregistry.com",
					Username: "anotheruser",
					Password: "anotherpassword",
				},
			},
			wantContents: `{
  "kind":"CredentialProviderResponse",
  "apiVersion":"credentialprovider.kubelet.k8s.io/v1",
  "cacheKeyType":"Image",
  "cacheDuration":"0s",
  "auth":{
    "myregistry.com": {"username": "myuser", "password": "mypassword"},
    "registry-1.docker.io": {"username": "myuser", "password": "mypassword"},
    "docker.io": {"username": "myuser", "password": "mypassword"},
    "anotherregistry.com": {"username": "anotheruser", "password": "anotherpassword"}
  }
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotContents, err := kubeletStaticCredentialProviderSecretContents(tt.configs)
			require.NoError(t, err)
			assert.Equal(t, tt.wantContents, gotContents)
		})
	}
}
