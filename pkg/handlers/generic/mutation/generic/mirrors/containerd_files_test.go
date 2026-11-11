// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cabpkv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
)

func Test_generateContainerdDefaultHostsFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		configs []containerdConfig
		want    *cabpkv1.File
		wantErr error
	}{
		{
			name: "ECR mirror image registry and no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Permissions: "0600",
				Content: `[host."https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com/v2"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
		},
		{
			name: "ECR mirror image registry with a path and no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com/myproject",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Permissions: "0600",
				Content: `[host."https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com/v2/myproject"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
		},
		{
			name: "Mirror image registry with a CA and an image registry with no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://mymirror.com",
					CACert: "mymirrorcert",
					Mirror: true,
				},
				{
					URL: "https://myregistry.com",
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Permissions: "0600",
				Content: `[host."https://mymirror.com/v2"]
  capabilities = ["pull", "resolve"]
  ca = "/etc/containerd/certs.d/mymirror.com/ca.crt"
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
		},
		{
			name: "Mirror image registry with a CA and an image registry with a CA",
			configs: []containerdConfig{
				{
					URL:    "https://mymirror.com",
					CACert: "mymirrorcert",
					Mirror: true,
				},
				{
					URL:    "https://myregistry.com",
					CACert: "myregistrycert",
				},
				{
					URL:    "https://172.100.0.10:5000/myproject",
					CACert: "myregistrycert",
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Permissions: "0600",
				Content: `[host."https://mymirror.com/v2"]
  capabilities = ["pull", "resolve"]
  ca = "/etc/containerd/certs.d/mymirror.com/ca.crt"
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
		},
		{
			name: "Image registry with a CA",
			configs: []containerdConfig{
				{
					URL:    "https://myregistry.com",
					CACert: "myregistrycert",
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := generateContainerdDefaultHostsFile(tt.configs)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, file)
		})
	}
}

func Test_generateRegistryCACertFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		configs []containerdConfig
		want    []cabpkv1.File
		wantErr error
	}{
		{
			name: "ECR mirror image registry with no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			want: nil,
		},
		{
			name: "Mirror image registry with CA certificate",
			configs: []containerdConfig{
				{
					URL:          "https://registry.example.com",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE-----",
					Mirror:       true,
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/registry.example.com/ca.crt",
					Permissions: "0600",
					ContentFrom: cabpkv1.FileSource{
						Secret: cabpkv1.SecretFileSource{
							Name: "my-registry-credentials-secret",
							Key:  "ca.crt",
						},
					},
				},
			},
		},
		{
			name: "Mirror image registry and registry with the same CA certificate",
			configs: []containerdConfig{
				{
					URL:          "https://registry.example.com/mirror",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE-----",
					Mirror:       true,
				},
				{
					URL:          "https://registry.example.com/library",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE-----",
					Mirror:       false,
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/registry.example.com/ca.crt",
					Permissions: "0600",
					ContentFrom: cabpkv1.FileSource{
						Secret: cabpkv1.SecretFileSource{
							Name: "my-registry-credentials-secret",
							Key:  "ca.crt",
						},
					},
				},
			},
		},
		{
			name: "Mirror image registry and registry with different CA certificates",
			configs: []containerdConfig{
				{
					URL:          "https://registry.example.com/mirror",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE-----",
					Mirror:       true,
				},
				{
					URL:          "https://registry.example.com/library",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE----------END CERTIFICATE-----",
					Mirror:       false,
				},
			},
			wantErr: ErrConflictingRegistryCACertificates,
		},
		{
			name: "Multiple image registries with different CA certificates",
			configs: []containerdConfig{
				{
					URL:          "https://registry.example.com/mirror",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE-----",
					Mirror:       false,
				},
				{
					URL:          "https://registry.example.com/library",
					CASecretName: "my-registry-credentials-secret",
					CACert:       "-----BEGIN CERTIFICATE----------END CERTIFICATE-----",
					Mirror:       false,
				},
			},
			wantErr: ErrConflictingRegistryCACertificates,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := generateRegistryCACertFiles(tt.configs)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, file)
		})
	}
}

func Test_generateContainerdRegistryConfigDropInFile(t *testing.T) {
	want := []cabpkv1.File{
		{
			Path:        "/etc/caren/containerd/patches/registry-config.toml",
			Permissions: "0600",
			Content: `[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
[plugins."io.containerd.cri.v1.images".registry]
  config_path = "/etc/containerd/certs.d"
`,
		},
	}
	file := generateContainerdRegistryConfigDropInFile()
	assert.Equal(t, want, file)
}

func Test_generateContainerdDefaultHostsFile_HTTP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		configs []containerdConfig
		want    *cabpkv1.File
	}{
		{
			name: "http mirror without CA",
			configs: []containerdConfig{
				{
					URL:    "http://10.111.45.34/library",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Permissions: "0600",
				Content: `[host."http://10.111.45.34/v2/library"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
		},
		{
			name: "http mirror with port",
			configs: []containerdConfig{
				{
					URL:    "http://my-registry.internal:5000",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Permissions: "0600",
				Content: `[host."http://my-registry.internal:5000/v2"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := generateContainerdDefaultHostsFile(tt.configs)
			require.NoError(t, err)
			assert.Equal(t, tt.want, file)
		})
	}
}

func Test_generateContainerdPerHostHostsFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		configs []containerdConfig
		want    []cabpkv1.File
		wantErr error
	}{
		{
			name: "http image registry without CA",
			configs: []containerdConfig{
				{
					URL: "http://myregistry.internal:5000",
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/myregistry.internal:5000/hosts.toml",
					Permissions: "0600",
					Content: `[host."http://myregistry.internal:5000"]
  capabilities = ["pull", "resolve"]
`,
				},
			},
		},
		{
			name: "http image registry with path is deduplicated to host",
			configs: []containerdConfig{
				{
					URL: "http://10.111.45.34/app-images",
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/10.111.45.34/hosts.toml",
					Permissions: "0600",
					Content: `[host."http://10.111.45.34"]
  capabilities = ["pull", "resolve"]
`,
				},
			},
		},
		{
			name: "two http image registries same host different paths deduplicated",
			configs: []containerdConfig{
				{
					URL: "http://10.111.45.34/library",
				},
				{
					URL: "http://10.111.45.34/app-images",
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/10.111.45.34/hosts.toml",
					Permissions: "0600",
					Content: `[host."http://10.111.45.34"]
  capabilities = ["pull", "resolve"]
`,
				},
			},
		},
		{
			name: "https image registry does not generate per-host hosts.toml",
			configs: []containerdConfig{
				{
					URL: "https://myregistry.com",
				},
			},
			want: nil,
		},
		{
			name: "http mirror does not generate per-host hosts.toml",
			configs: []containerdConfig{
				{
					URL:    "http://myregistry.com",
					Mirror: true,
				},
			},
			want: nil,
		},
		{
			name: "mixed https mirror and http registry",
			configs: []containerdConfig{
				{
					URL:    "https://mymirror.com",
					Mirror: true,
				},
				{
					URL: "http://myregistry.internal:5000",
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/myregistry.internal:5000/hosts.toml",
					Permissions: "0600",
					Content: `[host."http://myregistry.internal:5000"]
  capabilities = ["pull", "resolve"]
`,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			files, err := generateContainerdPerHostHostsFiles(tt.configs)
			require.ErrorIs(t, err, tt.wantErr)
			// Sort for deterministic comparison if needed, but single file cases are ordered.
			assert.Equal(t, tt.want, files)
		})
	}
}

func Test_needContainerdConfiguration_HTTP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		configs []containerdConfig
		want    bool
	}{
		{
			name: "http image registry without CA should need configuration",
			configs: []containerdConfig{
				{URL: "http://myregistry.internal:5000"},
			},
			want: true,
		},
		{
			name: "https image registry without CA should not need configuration",
			configs: []containerdConfig{
				{URL: "https://myregistry.com"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := needContainerdConfiguration(tt.configs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_generateFiles_HTTPExample(t *testing.T) {
	t.Parallel()
	configs := []containerdConfig{
		{
			URL:          "http://10.111.45.34/library",
			Mirror:       true,
			CASecretName: "registry-a-credentials",
			CACert:       "cert-a",
		},
		{
			URL:          "http://10.111.45.34/app-images",
			CASecretName: "registry-b-credentials",
			CACert:       "cert-b",
		},
	}
	_, err := generateFiles(configs)
	require.ErrorIs(t, err, ErrConflictingRegistryCACertificates)
}

func Test_generateFiles_HTTP_NoCAConflict(t *testing.T) {
	t.Parallel()
	configs := []containerdConfig{
		{
			URL:          "http://10.111.45.34/library",
			Mirror:       true,
			CASecretName: "registry-a-credentials",
			CACert:       "same-cert",
		},
		{
			URL:          "http://10.111.45.34/app-images",
			CASecretName: "registry-b-credentials",
			CACert:       "same-cert",
		},
	}
	files, err := generateFiles(configs)
	require.NoError(t, err)
	// Expect 4 files: _default/hosts.toml, per-host hosts.toml, ca.crt, registry-config.toml
	// But CA deduplicates to single ca.crt file for same host with same cert.
	assert.Len(t, files, 4)
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	assert.Contains(t, paths, "/etc/containerd/certs.d/_default/hosts.toml")
	assert.Contains(t, paths, "/etc/containerd/certs.d/10.111.45.34/hosts.toml")
	assert.Contains(t, paths, "/etc/containerd/certs.d/10.111.45.34/ca.crt")
	assert.Contains(t, paths, "/etc/caren/containerd/patches/registry-config.toml")
}

func Test_generateFiles_UserExample_NoCA(t *testing.T) {
	t.Parallel()
	// User example: global mirror http with path, and imageRegistry http with different path, no CA, different credentials
	configs := []containerdConfig{
		{
			URL:    "http://10.111.45.34/library",
			Mirror: true,
		},
		{
			URL: "http://10.111.45.34/app-images",
		},
	}
	files, err := generateFiles(configs)
	require.NoError(t, err)
	assert.Len(t, files, 3)
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	assert.Contains(t, paths, "/etc/containerd/certs.d/_default/hosts.toml")
	assert.Contains(t, paths, "/etc/containerd/certs.d/10.111.45.34/hosts.toml")
	assert.Contains(t, paths, "/etc/caren/containerd/patches/registry-config.toml")
	// Verify _default/hosts.toml contains http mirror
	var defaultHosts *cabpkv1.File
	for _, f := range files {
		if f.Path == "/etc/containerd/certs.d/_default/hosts.toml" {
			defaultHosts = &f
			break
		}
	}
	require.NotNil(t, defaultHosts)
	assert.Contains(t, defaultHosts.Content, `host."http://10.111.45.34/v2/library"`)
	// Verify per-host hosts.toml contains http host
	var perHost *cabpkv1.File
	for _, f := range files {
		if f.Path == "/etc/containerd/certs.d/10.111.45.34/hosts.toml" {
			perHost = &f
			break
		}
	}
	require.NotNil(t, perHost)
	assert.Contains(t, perHost.Content, `[host."http://10.111.45.34"]`)
}
