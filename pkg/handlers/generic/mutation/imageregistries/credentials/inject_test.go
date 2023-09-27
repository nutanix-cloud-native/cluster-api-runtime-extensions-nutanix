// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"testing"

	"k8s.io/apiserver/pkg/storage/names"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/testutils/capitest/request"
)

const (
	validSecretName = "myregistry-credentials"
)

func TestGeneratePatches(t *testing.T) {
	capitest.ValidateGeneratePatches(
		t,
		func() *imageRegistriesPatchHandler {
			// Server side apply does not work with the fake client, hack around it by pre-creating empty Secrets
			// https://github.com/kubernetes-sigs/controller-runtime/issues/2341
			fakeClient := fake.NewClientBuilder().WithObjects(
				newTestSecret(validSecretName, request.Namespace),
				newEmptySecret(
					credentialSecretName(request.KubeadmControlPlaneTemplateRequestObjectName), request.Namespace),
				newEmptySecret(
					credentialSecretName(request.KubeadmConfigTemplateRequestObjectName), request.Namespace),
			).Build()
			return NewPatch(fakeClient)
		},
		capitest.PatchTestDef{
			Name: "unset variable",
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmControlPlaneTemplate for ECR without a Secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.ImageRegistryCredentials{
						v1alpha1.ImageRegistryCredentialsResource{
							URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveLen(3),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
					ValueMatcher: gomega.HaveLen(1),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-bin-dir",
						"/etc/kubernetes/image-credential-provider/",
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-config",
						"/etc/kubernetes/image-credential-provider-config.yaml",
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmControlPlaneTemplate for registry with a Secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.ImageRegistryCredentials{
						v1alpha1.ImageRegistryCredentialsResource{
							URL: "https://my-registry.io",
							Secret: &corev1.ObjectReference{
								Name: validSecretName,
							},
						},
					},
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation:    "add",
					Path:         "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveLen(4),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
					ValueMatcher: gomega.HaveLen(1),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-bin-dir",
						"/etc/kubernetes/image-credential-provider/",
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-config",
						"/etc/kubernetes/image-credential-provider-config.yaml",
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmConfigTemplate for ECR without a Secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.ImageRegistryCredentials{
						v1alpha1.ImageRegistryCredentialsResource{
							URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
						},
					},
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
					Operation:    "add",
					Path:         "/spec/template/spec/files",
					ValueMatcher: gomega.HaveLen(3),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.HaveLen(1),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-bin-dir",
						"/etc/kubernetes/image-credential-provider/",
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "files added in KubeadmConfigTemplate for registry with a Secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.ImageRegistryCredentials{
						v1alpha1.ImageRegistryCredentialsResource{
							URL: "https://my-registry.io",
							Secret: &corev1.ObjectReference{
								Name: validSecretName,
							},
						},
					},
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
					Operation:    "add",
					Path:         "/spec/template/spec/files",
					ValueMatcher: gomega.HaveLen(4),
				},
				{
					Operation:    "add",
					Path:         "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.HaveLen(1),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs",
					ValueMatcher: gomega.HaveKeyWithValue(
						"image-credential-provider-bin-dir",
						"/etc/kubernetes/image-credential-provider/",
					),
				},
			},
		},
		capitest.PatchTestDef{
			Name: "error for a registry with no credentials",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					variableName,
					v1alpha1.ImageRegistryCredentials{
						v1alpha1.ImageRegistryCredentialsResource{
							URL: "https://my-registry.io",
						},
					},
				),
			},
			RequestItem:     request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedFailure: true,
		},
	)
}

func newTestSecret(name, namespace string) *corev1.Secret {
	secretData := map[string][]byte{
		"username": []byte("myuser"),
		"password": []byte("mypassword"),
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

func newEmptySecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}
}
