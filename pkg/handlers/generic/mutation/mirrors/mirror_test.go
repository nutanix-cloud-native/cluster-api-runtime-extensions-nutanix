// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
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
					Content: `[host."https://123456789.dkr.ecr.us-east-1.amazonaws.com/v2"]
  capabilities = ["pull", "resolve"]
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
`,
				},
			},
			wantErr: nil,
		},
		{
			name: "ECR image registry with a path and no CA certificate",
			config: &mirrorConfig{
				URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com/myproject",
			},
			want: []cabpkv1.File{
				{
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
					Content: `[host."https://myregistry.com/v2"]
  capabilities = ["pull", "resolve"]
  ca = "/etc/certs/mirror.pem"
  # don't rely on Containerd to add the v2/ suffix
  # there is a bug where it is added incorrectly for mirrors with a path
  override_path = true
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
			file, err := generateGlobalRegistryMirrorFile(tt.config)
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
		registry v1alpha1.GlobalImageRegistryMirror
		want     []cabpkv1.File
	}{
		{
			name: "Mirror registry with no CA certificate",
			config: &mirrorConfig{
				URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
			},
			registry: v1alpha1.GlobalImageRegistryMirror{
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
			registry: v1alpha1.GlobalImageRegistryMirror{
				URL: "https://registry.example.com",

				Credentials: &v1alpha1.RegistryCredentials{
					SecretRef: &v1alpha1.LocalObjectReference{
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

func Test_generateContainerdRegistryConfigDropInFile(t *testing.T) {
	want := []cabpkv1.File{
		{
			Path:        "/etc/containerd/cre.d/registry-config.toml",
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

func Test_generateContainerdApplyPatchesScript(t *testing.T) {
	wantFile := []cabpkv1.File{
		{
			Path:        "/etc/containerd/apply-patches.sh",
			Owner:       "",
			Permissions: "0700",
			Encoding:    "",
			Append:      false,
			//nolint:lll // just a long string
			Content: `#!/bin/bash
set -euo pipefail
IFS=$'\n\t'

declare -r TOML_MERGE_IMAGE="ghcr.io/mesosphere/toml-merge:v0.2.0"

if ! ctr --namespace k8s.io images check "name==${TOML_MERGE_IMAGE}" | grep "${TOML_MERGE_IMAGE}" >/dev/null; then
  ctr --namespace k8s.io images pull "${TOML_MERGE_IMAGE}"
fi

cleanup() {
  ctr images unmount "${tmp_ctr_mount_dir}" || true
}

trap 'cleanup' EXIT

readonly tmp_ctr_mount_dir="$(mktemp -d)"

ctr --namespace k8s.io images mount "${TOML_MERGE_IMAGE}" "${tmp_ctr_mount_dir}"
"${tmp_ctr_mount_dir}/usr/local/bin/toml-merge" -i --patch-file "/etc/containerd/cre.d/*.toml" /etc/containerd/config.toml
`,
		},
	}
	wantCmd := "/bin/bash /etc/containerd/apply-patches.sh"
	file, cmd, _ := generateContainerdApplyPatchesScript()
	assert.Equal(t, wantFile, file)
	assert.Equal(t, wantCmd, cmd)
}
