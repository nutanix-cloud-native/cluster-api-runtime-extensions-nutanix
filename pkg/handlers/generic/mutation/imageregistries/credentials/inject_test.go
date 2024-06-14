// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/imageregistries"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const (
	validSecretName = "myregistry-credentials"
)

func Test_needImageRegistryCredentialsConfiguration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		configs []providerConfig
		need    bool
		wantErr error
	}{
		{
			name: "ECR credentials",
			configs: []providerConfig{
				{URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			need: true,
		},
		{
			name: "registry with static credentials",
			configs: []providerConfig{{
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
			}},
			need: true,
		},
		{
			name: "ECR mirror",
			configs: []providerConfig{
				{
					URL:    "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			need: true,
		},
		{
			name: "mirror with static credentials",
			configs: []providerConfig{{
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
				Mirror:   true,
			}},
			need: true,
		},
		{
			name: "mirror with no credentials",
			configs: []providerConfig{{
				URL:    "https://myregistry.com",
				Mirror: true,
			}},
			need: false,
		},
		{
			name: "a registry with static credentials and a mirror with no credentials",
			configs: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
					Mirror:   true,
				},
				{
					URL:    "https://myregistry.com",
					Mirror: true,
				},
			},
			need: true,
		},
		{
			name: "registry with missing credentials",
			configs: []providerConfig{{
				URL: "https://myregistry.com",
			}},
			need:    false,
			wantErr: ErrCredentialsNotFound,
		},
	}

	for idx := range testCases {
		tt := testCases[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			need, err := needImageRegistryCredentialsConfiguration(tt.configs)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.need, need)
		})
	}
}

func TestImageRegistriesPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Image registry mutator suite")
}

var _ = Describe("Generate Image registry patches", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	patchGenerator := func() mutation.GeneratePatches {
		// Use direct client to allow patch handler to read objects created by tests.
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())
		return mutation.NewMetaGeneratePatchesHandler("", client, NewPatch(client)).(mutation.GeneratePatches)
	}

	testDefs := []struct {
		capitest.PatchTestDef
		expectOwnerReferenceOnSecrets bool
	}{
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "unset variable",
			},
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "files added in KubeadmControlPlaneTemplate for ECR without a Secret",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						[]v1alpha1.ImageRegistry{{
							URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
						}},
						imageregistries.VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/files",
						ValueMatcher: gomega.ContainElements(
							gomega.HaveKeyWithValue(
								"path", "/etc/caren/install-kubelet-credential-providers.sh",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/image-credential-provider-config.yaml",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/dynamic-credential-provider-config.yaml",
							),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
						ValueMatcher: gomega.ContainElement(
							"/bin/bash /etc/caren/install-kubelet-credential-providers.sh",
						),
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
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "files added in KubeadmControlPlaneTemplate for registry with a Secret",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						[]v1alpha1.ImageRegistry{{
							URL: "https://registry.example.com",
							Credentials: &v1alpha1.RegistryCredentials{
								SecretRef: &v1alpha1.LocalObjectReference{
									Name: validSecretName,
								},
							},
						}},
						imageregistries.VariableName,
					),
				},
				RequestItem: request.NewKubeadmControlPlaneTemplateRequest(
					"",
					"test-kubeadmconfigtemplate",
				),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/files",
						ValueMatcher: gomega.ContainElements(
							gomega.HaveKeyWithValue(
								"path", "/etc/caren/install-kubelet-credential-providers.sh",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/image-credential-provider-config.yaml",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/dynamic-credential-provider-config.yaml",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/static-image-credentials.json",
							),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/preKubeadmCommands",
						ValueMatcher: gomega.ContainElement(
							"/bin/bash /etc/caren/install-kubelet-credential-providers.sh",
						),
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
			expectOwnerReferenceOnSecrets: true,
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "files added in KubeadmConfigTemplate for ECR without a Secret",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						[]v1alpha1.ImageRegistry{{
							URL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
						}},
						imageregistries.VariableName,
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
								"path", "/etc/caren/install-kubelet-credential-providers.sh",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/image-credential-provider-config.yaml",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/dynamic-credential-provider-config.yaml",
							),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/preKubeadmCommands",
						ValueMatcher: gomega.ContainElement(
							"/bin/bash /etc/caren/install-kubelet-credential-providers.sh",
						),
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
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "files added in KubeadmConfigTemplate for registry with a Secret",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						[]v1alpha1.ImageRegistry{{
							URL: "https://registry.example.com",
							Credentials: &v1alpha1.RegistryCredentials{
								SecretRef: &v1alpha1.LocalObjectReference{
									Name: validSecretName,
								},
							},
						}},
						imageregistries.VariableName,
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
				RequestItem: request.NewKubeadmConfigTemplateRequest("", "test-kubeadmconfigtemplate"),
				ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
					{
						Operation: "add",
						Path:      "/spec/template/spec/files",
						ValueMatcher: gomega.ContainElements(
							gomega.HaveKeyWithValue(
								"path", "/etc/caren/install-kubelet-credential-providers.sh",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/image-credential-provider-config.yaml",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/dynamic-credential-provider-config.yaml",
							),
							gomega.HaveKeyWithValue(
								"path", "/etc/kubernetes/static-image-credentials.json",
							),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/preKubeadmCommands",
						ValueMatcher: gomega.ContainElement(
							"/bin/bash /etc/caren/install-kubelet-credential-providers.sh",
						),
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
			expectOwnerReferenceOnSecrets: true,
		},
		{
			PatchTestDef: capitest.PatchTestDef{
				Name: "error for a registry with no credentials",
				Vars: []runtimehooksv1.Variable{
					capitest.VariableWithValue(
						v1alpha1.ClusterConfigVariableName,
						[]v1alpha1.ImageRegistry{{
							URL: "https://registry.example.com",
						}},
						imageregistries.VariableName,
					),
				},
				RequestItem:     request.NewKubeadmControlPlaneTemplateRequestItem(""),
				ExpectedFailure: true,
			},
		},
	}

	// Create credentials secret before each test
	BeforeEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())

		gomega.Expect(client.Create(
			ctx,
			newRegistryCredentialsSecret(validSecretName, request.Namespace),
		)).To(gomega.BeNil())

		gomega.Expect(client.Create(
			ctx,
			&clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
			},
		)).To(gomega.BeNil())
	})

	// Delete credentials secret after each test
	AfterEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())

		gomega.Expect(client.Delete(
			ctx,
			newRegistryCredentialsSecret(validSecretName, request.Namespace),
		)).To(gomega.BeNil())

		gomega.Expect(client.Delete(
			ctx,
			&clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
			},
		)).To(gomega.BeNil())
	})
	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.PatchTestDef)

			// validate an OwnerReference was added to the user provided and generated Secrets
			if tt.expectOwnerReferenceOnSecrets {
				client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
				gomega.Expect(err).To(gomega.BeNil())

				for _, name := range []string{validSecretName, credentialSecretName(request.ClusterName)} {
					key := ctrlclient.ObjectKey{
						Namespace: request.Namespace,
						Name:      name,
					}
					secret := &corev1.Secret{}
					gomega.Expect(client.Get(
						context.Background(),
						key,
						secret,
					)).To(gomega.BeNil())
					gomega.Expect(secret.OwnerReferences).ToNot(gomega.BeEmpty())
				}
			}
		})
	}
})

func newRegistryCredentialsSecret(name, namespace string) *corev1.Secret {
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
