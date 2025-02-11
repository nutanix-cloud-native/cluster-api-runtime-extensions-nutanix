// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const (
	validMirrorCASecretName   = "myregistry-mirror-cacert"
	validMirrorNoCASecretName = "myregistry-mirror-no-cacert"
)

func TestMirrorsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Global mirror mutator suite")
}

var _ = Describe("Generate Global mirror patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		// Always initialize the testEnv variable in the closure.
		// This will allow ginkgo to initialize testEnv variable during test execution time.
		testEnv := helpers.TestEnv
		// use direct client instead of controller client. This will allow the patch handler to read k8s object
		// that are written by the tests.
		// Test cases writes credentials secret that the mutator handler reads.
		// Using direct client will enable reading it immediately.
		client, err := testEnv.GetK8sClient()
		gomega.Expect(err).To(gomega.BeNil())
		return mutation.NewMetaGeneratePatchesHandler("", client, NewPatch(client)).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "files added in KubeadmControlPlaneTemplate for registry with mirror without CA Certificate secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					v1alpha1.GlobalMirrorVariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmControlPlaneTemplate for registry with mirror with CA Certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &v1alpha1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					},
					v1alpha1.GlobalMirrorVariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/registry.example.com/ca.crt",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmControlPlaneTemplate for registry mirror with secret but missing CA certificate key",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &v1alpha1.LocalObjectReference{
								Name: validMirrorNoCASecretName,
							},
						},
					},
					v1alpha1.GlobalMirrorVariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmControlPlaneTemplate for image registry with CA Certificate secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.ImageRegistry{{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &v1alpha1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					}},
					v1alpha1.ImageRegistriesVariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/registry.example.com/ca.crt",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for registry mirror without CA certificate secret",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					v1alpha1.GlobalMirrorVariableName,
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
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for registry mirror with secret for CA certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &v1alpha1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					},
					v1alpha1.GlobalMirrorVariableName,
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
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/registry.example.com/ca.crt",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for registry mirror with secret but missing CA certificate key",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &v1alpha1.LocalObjectReference{
								Name: validMirrorNoCASecretName,
							},
						},
					},
					v1alpha1.GlobalMirrorVariableName,
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
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for image registry with secret for CA certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					[]v1alpha1.ImageRegistry{{
						URL: "https://registry.example.com:5050",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &v1alpha1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					}},
					v1alpha1.ImageRegistriesVariableName,
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
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/registry.example.com:5050/ca.crt",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
	}

	// Create credentials secret before each test
	BeforeEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClient()
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(client.Create(
			ctx,
			newMirrorSecretWithCA(validMirrorCASecretName, request.Namespace),
		)).To(gomega.BeNil())
		gomega.Expect(client.Create(
			ctx,
			newMirrorSecretWithoutCA(validMirrorNoCASecretName, request.Namespace),
		)).To(gomega.BeNil())
	})

	// Delete credentials secret after each test
	AfterEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClient()
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(client.Delete(
			ctx,
			newMirrorSecretWithCA(validMirrorCASecretName, request.Namespace),
		)).To(gomega.BeNil())
		gomega.Expect(client.Delete(
			ctx,
			newMirrorSecretWithoutCA(validMirrorNoCASecretName, request.Namespace),
		)).To(gomega.BeNil())
	})

	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})

func newMirrorSecretWithCA(name, namespace string) *corev1.Secret {
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

func newMirrorSecretWithoutCA(name, namespace string) *corev1.Secret {
	secretData := map[string][]byte{
		"username": []byte("user"),
		"password": []byte("pass"),
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

func Test_needContainerdConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		configs []containerdConfig
		want    bool
	}{
		{
			name: "ECR mirror image registry with no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			want: true,
		},
		{
			name: "ECR mirror image registry with a path and no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com/myproject",
					Mirror: true,
				},
			},
			want: true,
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
			want: true,
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
			},
			want: true,
		},
		{
			name: "Image registry with no CA certificate",
			configs: []containerdConfig{
				{
					URL: "https://myregistry.com",
				},
			},
			want: false,
		},
	}
	for idx := range tests {
		tt := tests[idx]
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := needContainerdConfiguration(tt.configs)
			assert.Equal(t, tt.want, got)
		})
	}
}
