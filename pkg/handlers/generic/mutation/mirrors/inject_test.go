// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const (
	validMirrorCASecretName = "myregistry-mirror-cacert"
	//nolint:gosec // Does not contain hard coded credentials.
	cpRegistryAsMirrorCreds = "kubeadmControlPlaneRegistryAsMirrorCreds"
	//nolint:gosec // Does not contain hard coded credentials.
	workerRegistryAsMirrorCreds = "kubeadmConfigTemplateRegistryAsMirrorCreds"
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
		return mutation.NewMetaGeneratePatchesHandler("", NewPatch(client)).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "files added in KubeadmControlPlaneTemplate for registry with mirror without CA Certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					GlobalMirrorVariableName,
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
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/cre.d/registry-config.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/apply-patches.sh",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/restart.sh",
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElements(
						"/bin/bash /etc/containerd/apply-patches.sh",
						"/bin/bash /etc/containerd/restart.sh",
					),
				},
			},
		},
		{
			Name: "files added in KubeadmControlPlaneTemplate for registry with mirror with CA Certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &corev1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					},
					GlobalMirrorVariableName,
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
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/cre.d/registry-config.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/apply-patches.sh",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/restart.sh",
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElements(
						"/bin/bash /etc/containerd/apply-patches.sh",
						"/bin/bash /etc/containerd/restart.sh",
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for registry mirror wihthout CA certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					GlobalMirrorVariableName,
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
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/cre.d/registry-config.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/apply-patches.sh",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/restart.sh",
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElements(
						"/bin/bash /etc/containerd/apply-patches.sh",
						"/bin/bash /etc/containerd/restart.sh",
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for registry mirror with secret for CA certificate",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					v1alpha1.GlobalImageRegistryMirror{
						URL: "https://registry.example.com",
						Credentials: &v1alpha1.RegistryCredentials{
							SecretRef: &corev1.LocalObjectReference{
								Name: validMirrorCASecretName,
							},
						},
					},
					GlobalMirrorVariableName,
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
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/cre.d/registry-config.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/apply-patches.sh",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/restart.sh",
						),
					),
				},
				{
					Operation: "add",
					Path:      "/spec/template/spec/preKubeadmCommands",
					ValueMatcher: gomega.ContainElements(
						"/bin/bash /etc/containerd/apply-patches.sh",
						"/bin/bash /etc/containerd/restart.sh",
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
			newMirrorSecret(validMirrorCASecretName, request.Namespace),
		)).To(gomega.BeNil())
	})

	// Delete credentials secret after each test
	AfterEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClient()
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(client.Delete(
			ctx,
			newMirrorSecret(validMirrorCASecretName, request.Namespace),
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
