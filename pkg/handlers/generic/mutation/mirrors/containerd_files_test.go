// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
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
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `[host."https://123456789.dkr.ecr.us-east-1.amazonaws.com/v2"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
			wantErr: nil,
		},
		{
			name: "ECR mirror image registry with a path and no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com/myproject",
					Mirror: true,
				},
			},
			want: &cabpkv1.File{
				Path:        "/etc/containerd/certs.d/_default/hosts.toml",
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `[host."https://123456789.dkr.ecr.us-east-1.amazonaws.com/v2/myproject"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
			wantErr: nil,
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
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `[host."https://mymirror.com/v2"]
  capabilities = ["pull", "resolve"]
  ca = "/etc/containerd/certs.d/mymirror.com/ca.crt"
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
			wantErr: nil,
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
				Owner:       "",
				Permissions: "0600",
				Encoding:    "",
				Append:      false,
				Content: `[host."https://mymirror.com/v2"]
  capabilities = ["pull", "resolve"]
  ca = "/etc/containerd/certs.d/mymirror.com/ca.crt"
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
			},
			wantErr: nil,
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

			wantErr: nil,
		},
	}
	for idx := range tests {
		tt := tests[idx]
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
	}{
		{
			name: "ECR mirror image registry with no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
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
					Mirror:       true,
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/registry.example.com/ca.crt",
					Owner:       "",
					Permissions: "0600",
					Encoding:    "",
					Append:      false,
					ContentFrom: &cabpkv1.FileSource{
						Secret: cabpkv1.SecretFileSource{
							Name: "my-registry-credentials-secret",
							Key:  "ca.crt",
						},
					},
				},
			},
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := generateRegistryCACertFiles(tt.configs)
			require.NoError(t, err)
			assert.Equal(t, tt.want, file)
		})
	}
}

func Test_generateContainerdRegistryConfigDropInFile(t *testing.T) {
	want := []cabpkv1.File{
		{
			Path:        "/etc/caren/containerd/patches/registry-config.toml",
			Owner:       "",
			Permissions: "0600",
			Encoding:    "",
			Append:      false,
			Content: `[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "/etc/containerd/certs.d"
`,
		},
	}
	file := generateContainerdRegistryConfigDropInFile()
	assert.Equal(t, want, file)
}
