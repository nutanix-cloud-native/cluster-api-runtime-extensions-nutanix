// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"context"
	"fmt"
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
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const (
	validMirrorCASecretName   = "myregistry-mirror-cacert"
	validMirrorNoCASecretName = "myregistry-mirror-no-cacert"

	registryAddonCAForCluster = "test-cluster-registry-addon-ca"
)

func TestMirrorsPatch(t *testing.T) {
	gomega.RegisterFailHandler(Fail)
	RunSpecs(t, "Global mirror mutator suite")
}

var _ = Describe("Generate Global mirror patches", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))

	patchGenerator := func() mutation.GeneratePatches {
		// Always initialize the testEnv variable in the closure.
		// This will allow ginkgo to initialize testEnv variable during test execution time.
		testEnv := helpers.TestEnv
		// use direct client instead of controller client. This will allow the patch handler to read k8s object
		// that are written by the tests.
		// Test cases writes credentials secret that the mutator handler reads.
		// Using direct client will enable reading it immediately.
		client, err := testEnv.GetK8sClientWithScheme(clientScheme)
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
						URL: "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
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
						URL: "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
					},
					v1alpha1.GlobalMirrorVariableName,
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
		{
			Name: "files added in KubeadmControlPlaneTemplate for registry addon",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.RegistryAddon{},
					[]string{"addons", v1alpha1.RegistryAddonVariableName}...,
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
							"path", "/etc/containerd/certs.d/192.168.0.20/ca.crt",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/caren/containerd/patches/registry-config.toml",
						),
					),
				},
			},
		},
		{
			Name: "files added in KubeadmConfigTemplate for registry addon",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					v1alpha1.ClusterConfigVariableName,
					v1alpha1.RegistryAddon{},
					[]string{"addons", v1alpha1.RegistryAddonVariableName}...,
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
					ValueMatcher: gomega.HaveExactElements(
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/_default/hosts.toml",
						),
						gomega.HaveKeyWithValue(
							"path", "/etc/containerd/certs.d/192.168.0.20/ca.crt",
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
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		gomega.Expect(err).To(gomega.BeNil())
		gomega.Expect(client.Create(
			ctx,
			newRegistrySecretWithCA(validMirrorCASecretName),
		)).To(gomega.BeNil())
		gomega.Expect(client.Create(
			ctx,
			newRegistrySecretWithoutCA(validMirrorNoCASecretName),
		)).To(gomega.BeNil())
		gomega.Expect(client.Create(
			ctx,
			newRegistrySecretWithCA(registryAddonCAForCluster),
		)).To(gomega.BeNil())

		gomega.Expect(client.Create(
			ctx,
			&clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: request.Namespace,
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
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
			newRegistrySecretWithCA(validMirrorCASecretName),
		)).To(gomega.BeNil())
		gomega.Expect(client.Delete(
			ctx,
			newRegistrySecretWithoutCA(validMirrorNoCASecretName),
		)).To(gomega.BeNil())
		gomega.Expect(client.Delete(
			ctx,
			newRegistrySecretWithCA(registryAddonCAForCluster),
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
	for _, tt := range testDefs {
		It(tt.Name, func() {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)
		})
	}
})

func Test_containerdConfigFromRegistryAddon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		c       ctrlclient.Client
		cluster *clusterv1.Cluster
		want    containerdConfig
		wantErr error
	}{
		{
			name: "valid input with a CA certificate",
			c: fake.NewClientBuilder().WithObjects(
				newRegistrySecretWithCA(registryAddonCAForCluster),
			).Build(),
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
					},
				},
			},
			want: containerdConfig{
				URL:          "https://192.168.0.20",
				Mirror:       true,
				CASecretName: "test-cluster-registry-addon-ca",
				CACert:       "myCACert",
			},
		},
		{
			name: "error: missing Services CIDR",
			c:    fake.NewClientBuilder().Build(),
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{},
				},
			},
			wantErr: fmt.Errorf(
				"error getting service IP for the registry addon: " +
					"error getting a service IP for a cluster: " +
					"unexpected empty service Subnets",
			),
		},
		{
			name: "error: missing certificate in the secret",
			// The suffix "-ca" is misleading here because we expect the generated secret to always have a CA.
			c: fake.NewClientBuilder().WithObjects(
				newRegistrySecretWithoutCA("test-cluster-registry-addon-ca"),
			).Build(),
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: corev1.NamespaceDefault,
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.1/16"},
						},
					},
				},
			},
			wantErr: fmt.Errorf("CA certificate not found in the secret"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := containerdConfigFromRegistryAddon(context.Background(), tt.c, tt.cluster)
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
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
					URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com",
					Mirror: true,
				},
			},
			want: true,
		},
		{
			name: "ECR mirror image registry with a path and no CA certificate",
			configs: []containerdConfig{
				{
					URL:    "https://o-0123456789.dkr.ecr.us-east-1.amazonaws.com/myproject",
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := needContainerdConfiguration(tt.configs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func newRegistrySecretWithCA(name string) *corev1.Secret {
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
			Namespace: corev1.NamespaceDefault,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}
}

func newRegistrySecretWithoutCA(name string) *corev1.Secret {
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
			Namespace: corev1.NamespaceDefault,
		},
		Data: secretData,
		Type: corev1.SecretTypeOpaque,
	}
}
