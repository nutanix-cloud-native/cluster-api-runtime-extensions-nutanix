// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
)

func Test_generateDefaultRegistryMirrorFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		config  *mirrorConfig
		want    []cabpkv1.File
		wantErr error
	}{
		{
			name:   "ECR image registry and no CA certificate",
			config: &mirrorConfig{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/_default/hosts.toml",
					Owner:       "",
					Permissions: "0600",
					Encoding:    "",
					Append:      false,
					Content: `[host."https://123456789.dkr.ecr.us-east-1.amazonaws.com"]
  capabilities = ["pull", "resolve"]
`,
				},
			},
			wantErr: nil,
		},
		{
			name: "image registry with CA certificates",
			config: &mirrorConfig{
				URL:    "https://myregistry.com",
				CACert: "mycacert",
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/containerd/certs.d/_default/hosts.toml",
					Owner:       "",
					Permissions: "0600",
					Encoding:    "",
					Append:      false,
					Content: `[host."https://myregistry.com"]
  capabilities = ["pull", "resolve"]
  ca = "/etc/certs/mirror.pem"
`,
				},
			},
			wantErr: nil,
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := generateDefaultRegistryMirrorFile(tt.config)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, file)
		})
	}
}

func Test_generateMirrorCACertFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		config   *mirrorConfig
		registry v1alpha1.ImageRegistry
		want     []cabpkv1.File
	}{
		{
			name: "Mirror registry with no CA certificate",
			config: &mirrorConfig{
				URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
			},
			registry: v1alpha1.ImageRegistry{
				URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
			},
			want: nil,
		},
		{
			name: "Mirror registry with CA certificate",
			config: &mirrorConfig{
				URL:    "https://myregistry.com",
				CACert: "mycacert",
			},
			registry: v1alpha1.ImageRegistry{
				URL: "https://myregistry.com",
				Mirror: &v1alpha1.RegistryMirror{
					SecretRef: &v1.ObjectReference{
						Name: "my-registry-credentials-secret",
					},
				},
			},
			want: []cabpkv1.File{
				{
					Path:        "/etc/certs/mirror.pem",
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
			file := generateMirrorCACertFile(tt.config, tt.registry)
			assert.Equal(t, tt.want, file)
		})
	}
}
