// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

const (
	validMirrorCASecretName = "myregistry-mirror-cacert"
	//nolint:gosec // Does not contain hard coded credentials.
	cpRegistryAsMirrorCreds = "kubeadmControlPlaneRegistryAsMirrorCreds"
	//nolint:gosec // Does not contain hard coded credentials.
	workerRegistryAsMirrorCreds = "kubeadmConfigTemplateRegistryAsMirrorCreds"
)

func TestGeneratePatches(
	t *testing.T,
	generatorFunc func() mutation.GeneratePatches,
	fakeClient client.Client,
	variableName string,
	variablePath ...string,
) {
	t.Helper()

	require.NoError(
		t,
		fakeClient.Create(
			context.Background(),
			newMirrorSecret(validMirrorCASecretName, request.Namespace),
		),
	)

	capitest.ValidateGeneratePatches(
		t,
		generatorFunc,
		capitest.PatchTestDef{
			Name: "files added in KubeadmControlPlaneTemplate for registry with mirror without CA Certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					variablePath...,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmControlPlaneTemplate for registry with mirror with CA Certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &corev1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					},
					variablePath...,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequest("", cpRegistryAsMirrorCreds),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/certs/mirror.pem",
						),
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmConfigTemplate for registry mirror wihthout CA certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					variablePath...,
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmConfigTemplate for registry mirror with secret for CA certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &corev1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					},
					variablePath...,
				),
				capitest.VariableWithValue(
					"builtin",
					map[string]any{
						"machineDeployment": map[string]any{
							"class": names.SimpleNameGenerator.GenerateName("worker-"),
						},
					},
				),
			},
			RequestItem: request.NewKubeadmConfigTemplateRequest("", workerRegistryAsMirrorCreds),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/files",
					ValueMatcher: gomega.ContainElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/certs/mirror.pem",
						),
					),
				},
			},
		},
	)
}

func newMirrorSecret(name, namespace string) *corev1.Secret {
	secretData := map[string][]byte{
		"ca.crt": []byte("myCACert"),
	}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}
}
