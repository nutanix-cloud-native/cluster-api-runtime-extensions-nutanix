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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/util"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const (
	validSecretName = "myregistry-credentials"
)

func Test_providerConfigsThatNeedConfiguration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		configs  []providerConfig
		expected []providerConfig
		wantErr  error
	}{
		{
			name: "ECR registry with no credentials",
			configs: []providerConfig{
				{URL: "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
			expected: []providerConfig{
				{URL: "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com"},
			},
		},
		{
			name: "registry with static credentials",
			configs: []providerConfig{{
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
			}},
			expected: []providerConfig{{
				URL:      "https://myregistry.com",
				Username: "myuser",
				Password: "mypassword",
			}},
		},
		{
			name: "ECR mirror with no credentials",
			configs: []providerConfig{{
				URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
				Mirror: true,
			}},
			expected: []providerConfig{{
				URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
				Mirror: true,
			}},
		},
		{
			name: "mirror with static credentials",
			configs: []providerConfig{{
				URL:      "https://mymirror.com",
				Username: "myuser",
				Password: "mypassword",
				Mirror:   true,
			}},
			expected: []providerConfig{{
				URL:      "https://mymirror.com",
				Username: "myuser",
				Password: "mypassword",
				Mirror:   true,
			}},
		},
		{
			name: "mirror with no credentials",
			configs: []providerConfig{{
				URL:    "https://mymirror.com",
				Mirror: true,
			}},
			expected: nil,
		},
		{
			name: "a registry with static credentials and a mirror with no credentials",
			configs: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
					Mirror:   false,
				},
				{
					URL:    "https://mymirror.com",
					Mirror: true,
				},
			},
			expected: []providerConfig{
				{
					URL:      "https://myregistry.com",
					Username: "myuser",
					Password: "mypassword",
					Mirror:   false,
				},
			},
		},
		{
			name: "a registry with missing credentials and a mirror with no credentials",
			configs: []providerConfig{
				{
					URL:    "https://myregistry.com",
					Mirror: false,
				},
				{
					URL:    "https://mymirror.com",
					Mirror: true,
				},
			},
			wantErr: ErrCredentialsNotFound,
		},
		{
			name: "registry with missing credentials",
			configs: []providerConfig{{
				URL: "https://myregistry.com",
			}},
			wantErr: ErrCredentialsNotFound,
		},
		{
			name: "registry with missing credentials but with a CA",
			configs: []providerConfig{{
				URL:       "https://myregistry.com",
				HasCACert: true,
			}},
		},
		{
			name: "mirror with missing credentials but with a CA",
			configs: []providerConfig{{
				URL:       "https://mymirror.com",
				HasCACert: true,
				Mirror:    true,
			}},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expected, err := providerConfigsThatNeedConfiguration(tt.configs)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.expected, expected)
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
							URL: "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
						}},
						v1alpha1.ImageRegistriesVariableName,
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
						Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs/1", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-bin-dir"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider/"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs/2", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-config"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider-config.yaml"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs/1", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-bin-dir"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider/"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs/2", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-config"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider-config.yaml"),
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
						v1alpha1.ImageRegistriesVariableName,
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
						Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs/1", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-bin-dir"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider/"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/initConfiguration/nodeRegistration/kubeletExtraArgs/2", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-config"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider-config.yaml"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs/1", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-bin-dir"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider/"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/kubeadmConfigSpec/joinConfiguration/nodeRegistration/kubeletExtraArgs/2", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-config"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider-config.yaml"),
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
							URL: "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
						}},
						v1alpha1.ImageRegistriesVariableName,
					),
					capitest.VariableWithValue(
						runtimehooksv1.BuiltinsName,
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
						Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/1", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-bin-dir"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider/"),
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
						v1alpha1.ImageRegistriesVariableName,
					),
					capitest.VariableWithValue(
						runtimehooksv1.BuiltinsName,
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
						Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/1", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-bin-dir"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider/"),
						),
					},
					{
						Operation: "add",
						Path:      "/spec/template/spec/joinConfiguration/nodeRegistration/kubeletExtraArgs/2", //nolint:lll // Just a long line.
						ValueMatcher: gomega.SatisfyAll(
							gomega.HaveKeyWithValue("name", "image-credential-provider-config"),
							gomega.HaveKeyWithValue("value", "/etc/kubernetes/image-credential-provider-config.yaml"),
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
						v1alpha1.ImageRegistriesVariableName,
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
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{
						ClassRef: clusterv1.ClusterClassRef{Name: "test"},
						Version:  "v1.30.0",
					},
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
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{
						ClassRef: clusterv1.ClusterClassRef{Name: "test"},
						Version:  "v1.30.0",
					},
				},
			},
		)).To(gomega.BeNil())
	})
	// create test node for each case
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt.PatchTestDef)

			// validate an OwnerReference was added to the user provided and generated Secrets
			if tt.expectOwnerReferenceOnSecrets {
				client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
				gomega.Expect(err).To(gomega.BeNil())

				// get the Cluster to use for the owner reference assertion
				clusterKey := ctrlclient.ObjectKey{
					Namespace: request.Namespace,
					Name:      request.ClusterName,
				}
				cluster := &clusterv1.Cluster{}
				gomega.Expect(client.Get(
					context.Background(),
					clusterKey,
					cluster,
				)).To(gomega.BeNil())
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

					// assert the owner reference with the Cluster was added to the Secret
					gomega.Expect(secret.OwnerReferences).ToNot(gomega.BeEmpty())
					ownerRef := metav1.OwnerReference{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       cluster.Kind,
						UID:        cluster.UID,
						Name:       cluster.Name,
					}
					util.HasOwnerRef(secret.OwnerReferences, ownerRef)
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
