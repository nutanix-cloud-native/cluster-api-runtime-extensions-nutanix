// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
)

func Test_templateKubeletCredentialProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		credentials []providerConfig
		want        *cabpkv1.File
		wantErr     error
	}{
		{
			name: "ECR image registry",
			credentials: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/image-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: dynamic-credential-provider
  args:
  - get-credentials
  - -c
  - /etc/kubernetes/dynamic-credential-provider-config.yaml
  matchImages:
  - "123456789.dkr.ecr.us-east-1.amazonaws.com"
  - "*"
  - "*.*"
  - "*.*.*"
  - "*.*.*.*"
  - "*.*.*.*.*"
  - "*.*.*.*.*.*"
  defaultCacheDuration: "0s"
  apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "image registry with static config",
			credentials: []providerConfig{{
				URL:      "https://myregistry.com:5000/myproject",
				Username: "myuser",
				Password: "mypassword",
			}},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/image-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: dynamic-credential-provider
  args:
  - get-credentials
  - -c
  - /etc/kubernetes/dynamic-credential-provider-config.yaml
  matchImages:
  - "myregistry.com:5000/myproject"
  - "*"
  - "*.*"
  - "*.*.*"
  - "*.*.*.*"
  - "*.*.*.*.*"
  - "*.*.*.*.*.*"
  defaultCacheDuration: "0s"
  apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "docker.io registry with static credentials",
			credentials: []providerConfig{{
				URL:      "https://registry-1.docker.io",
				Username: "myuser",
				Password: "mypassword",
			}},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/image-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: dynamic-credential-provider
  args:
  - get-credentials
  - -c
  - /etc/kubernetes/dynamic-credential-provider-config.yaml
  matchImages:
  - "registry-1.docker.io"
  - "docker.io"
  - "*"
  - "*.*"
  - "*.*.*"
  - "*.*.*.*"
  - "*.*.*.*.*"
  - "*.*.*.*.*.*"
  defaultCacheDuration: "0s"
  apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "multiple image registries with static config",
			credentials: []providerConfig{{
				URL:      "https://myregistry.com:5000/myproject",
				Username: "myuser",
				Password: "mypassword",
			}, {
				URL:      "https://myotherregistry.com:5000/myproject",
				Username: "otheruser",
				Password: "otherpassword",
			}},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/image-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
- name: dynamic-credential-provider
  args:
  - get-credentials
  - -c
  - /etc/kubernetes/dynamic-credential-provider-config.yaml
  matchImages:
  - "myregistry.com:5000/myproject"
  - "myotherregistry.com:5000/myproject"
  - "*"
  - "*.*"
  - "*.*.*"
  - "*.*.*.*"
  - "*.*.*.*.*"
  - "*.*.*.*.*.*"
  defaultCacheDuration: "0s"
  apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := templateKubeletCredentialProviderConfig(tt.credentials)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, file)
		})
	}
}

func Test_templateDynamicCredentialProviderConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		credentials []providerConfig
		want        *cabpkv1.File
		wantErr     error
	}{
		{
			name: "ECR image registry",
			credentials: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/dynamic-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: credentialprovider.d2iq.com/v1alpha1
kind: DynamicCredentialProviderConfig
credentialProviderPluginBinDir: /etc/kubernetes/image-credential-provider/
credentialProviders:
  apiVersion: kubelet.config.k8s.io/v1
  kind: CredentialProviderConfig
  providers:
  - name: ecr-credential-provider
    args:
    - get-credentials
    matchImages:
    - "123456789.dkr.ecr.us-east-1.amazonaws.com"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "image registry with static credentials",
			credentials: []providerConfig{{
				URL:      "https://myregistry.com:5000/myproject",
				Username: "myuser",
				Password: "mypassword",
			}},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/dynamic-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: credentialprovider.d2iq.com/v1alpha1
kind: DynamicCredentialProviderConfig
credentialProviderPluginBinDir: /etc/kubernetes/image-credential-provider/
credentialProviders:
  apiVersion: kubelet.config.k8s.io/v1
  kind: CredentialProviderConfig
  providers:
  - name: static-credential-provider
    args:
    - /etc/kubernetes/static-image-credentials.json
    matchImages:
    - "myregistry.com:5000/myproject"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "docker.io registry with static credentials",
			credentials: []providerConfig{{
				URL:      "https://registry-1.docker.io",
				Username: "myuser",
				Password: "mypassword",
			}},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/dynamic-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: credentialprovider.d2iq.com/v1alpha1
kind: DynamicCredentialProviderConfig
credentialProviderPluginBinDir: /etc/kubernetes/image-credential-provider/
credentialProviders:
  apiVersion: kubelet.config.k8s.io/v1
  kind: CredentialProviderConfig
  providers:
  - name: static-credential-provider
    args:
    - /etc/kubernetes/static-image-credentials.json
    matchImages:
    - "registry-1.docker.io"
    - "docker.io"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "multiple registries",
			credentials: []providerConfig{{
				URL:      "https://registry-1.docker.io",
				Username: "myuser",
				Password: "mypassword",
			}, {
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
			}, {
				URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
			}, {
				URL:      "https://anotherregistry.com",
				Username: "anotheruser",
				Password: "anotherpassword",
			}},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/dynamic-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: credentialprovider.d2iq.com/v1alpha1
kind: DynamicCredentialProviderConfig
credentialProviderPluginBinDir: /etc/kubernetes/image-credential-provider/
credentialProviders:
  apiVersion: kubelet.config.k8s.io/v1
  kind: CredentialProviderConfig
  providers:
  - name: ecr-credential-provider
    args:
    - get-credentials
    matchImages:
    - "123456789.dkr.ecr.us-east-1.amazonaws.com"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
  - name: static-credential-provider
    args:
    - /etc/kubernetes/static-image-credentials.json
    matchImages:
    - "anotherregistry.com"
    - "myregistry.com"
    - "registry-1.docker.io"
    - "docker.io"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "ECR global mirror image registry",
			credentials: []providerConfig{
				{
					URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
				},
				{
					URL:    "https://98765432.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/dynamic-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: credentialprovider.d2iq.com/v1alpha1
kind: DynamicCredentialProviderConfig
mirror:
  endpoint: 98765432.dkr.ecr.us-east-1.amazonaws.com
  credentialsStrategy: MirrorCredentialsFirst
credentialProviderPluginBinDir: /etc/kubernetes/image-credential-provider/
credentialProviders:
  apiVersion: kubelet.config.k8s.io/v1
  kind: CredentialProviderConfig
  providers:
  - name: ecr-credential-provider
    args:
    - get-credentials
    matchImages:
    - "123456789.dkr.ecr.us-east-1.amazonaws.com"
    - "98765432.dkr.ecr.us-east-1.amazonaws.com"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
		{
			name: "Global mirror image registry with static credentials",
			credentials: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
				},
				{
					URL:      "https://mymirror.com",
					Username: "mirroruser",
					Password: "mirrorpassword",
					Mirror:   true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/kubernetes/dynamic-credential-provider-config.yaml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `apiVersion: credentialprovider.d2iq.com/v1alpha1
kind: DynamicCredentialProviderConfig
mirror:
  endpoint: mymirror.com
  credentialsStrategy: MirrorCredentialsFirst
credentialProviderPluginBinDir: /etc/kubernetes/image-credential-provider/
credentialProviders:
  apiVersion: kubelet.config.k8s.io/v1
  kind: CredentialProviderConfig
  providers:
  - name: static-credential-provider
    args:
    - /etc/kubernetes/static-image-credentials.json
    matchImages:
    - "mymirror.com"
    - "myregistry.com"
    defaultCacheDuration: "0s"
    apiVersion: credentialprovider.kubelet.k8s.io/v1
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := templateDynamicCredentialProviderConfig(tt.credentials)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, file)
		})
	}
}
