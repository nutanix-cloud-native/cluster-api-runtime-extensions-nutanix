// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/testutils/capitest/request"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/helpers"
)

const (
	testToken                      = "testAESConfigKey" //nolint:gosec // Does not contain hard coded credentials.
	testEncryptionConfigSecretData = `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- providers:
  - aescbc:
      keys:
      - name: key1
        secret: dGVzdEFFU0NvbmZpZ0tleQ==
  resources:
  - secrets
  - configmaps`
)

func testTokenGenerator() ([]byte, error) {
	return []byte(testToken), nil
}

func TestEncryptionConfigurationPatch(t *testing.T) {
	RegisterFailHandler(Fail)
	format.TruncatedDiff = false
	RunSpecs(t, "Encryption configuration mutator suite")
}

var _ = Describe("Generate Encryption configuration patches", func() {
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	patchGenerator := func() mutation.GeneratePatches {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			client,
			NewPatch(client, testTokenGenerator)).(mutation.GeneratePatches)
	}

	encryptionVar := []runtimehooksv1.Variable{
		capitest.VariableWithValue(
			clusterconfig.MetaVariableName,
			carenv1.EncryptionAtRest{
				Providers: []carenv1.EncryptionProviders{
					{
						AESCBC: &carenv1.AESConfiguration{},
					},
				},
			},
			VariableName,
		),
	}
	encryptionMatchers := []capitest.JSONPatchMatcher{
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/files",
			ValueMatcher: ContainElements(
				SatisfyAll(
					HaveKeyWithValue(
						"path", "/etc/kubernetes/pki/encryptionconfig.yaml",
					),
					HaveKeyWithValue(
						"permissions", "0640",
					),
					HaveKeyWithValue(
						"contentFrom",
						map[string]interface{}{
							"secret": map[string]interface{}{
								"key":  "config",
								"name": defaultEncryptionSecretName(request.ClusterName),
							},
						},
					),
				),
			),
		},
		{
			Operation: "add",
			Path:      "/spec/template/spec/kubeadmConfigSpec/clusterConfiguration",
			ValueMatcher: HaveKeyWithValue(
				"apiServer",
				HaveKeyWithValue(
					"extraArgs",
					map[string]interface{}{
						"encryption-provider-config": "/etc/kubernetes/pki/encryptionconfig.yaml",
					},
				),
			),
		},
	}

	// Create cluster before each test
	BeforeEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		Expect(client.Create(
			ctx,
			&clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: metav1.NamespaceDefault,
				},
			},
		)).To(BeNil())
	})

	// Delete cluster after each test
	AfterEach(func(ctx SpecContext) {
		client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
		Expect(err).To(BeNil())

		Expect(client.Delete(
			ctx,
			&clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      request.ClusterName,
					Namespace: metav1.NamespaceDefault,
				},
			},
		)).To(BeNil())
	})

	// Test that encryption config patch is generated without recreating the default encryption secret.
	// The Mutate function must be ideompotent and always generate patch in success cases.
	noOpEncryptionConfigDef := capitest.PatchTestDef{
		Name:                  "skip creating default encryption config secret if it already exists",
		Vars:                  encryptionVar,
		RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: encryptionMatchers,
	}

	Context("Default encryption provider secret already exists", func() {
		// encryption secret was created earlier
		BeforeEach(func(ctx SpecContext) {
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			Expect(client.Create(
				ctx,
				testEncryptionSecretObj(),
			)).To(BeNil())
		})
		// delete encryption configuration after the test
		AfterEach(func(ctx SpecContext) {
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			Expect(client.Delete(
				ctx,
				testEncryptionSecretObj(),
			)).To(BeNil())
		})
		It(noOpEncryptionConfigDef.Name, func(ctx SpecContext) {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &noOpEncryptionConfigDef)
		})
	})

	// Test that encryption configuration secret is generated and patched on kubeadmconfig spec.
	patchEncryptionConfigDef := capitest.PatchTestDef{
		Name:                  "files added in KubeadmControlPlaneTemplate for Encryption Configuration",
		Vars:                  encryptionVar,
		RequestItem:           request.NewKubeadmControlPlaneTemplateRequestItem(""),
		ExpectedPatchMatchers: encryptionMatchers,
	}

	Context("Default encryption configuration secret is created by the patch", func() {
		It(patchEncryptionConfigDef.Name, func(ctx SpecContext) {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &patchEncryptionConfigDef)

			// assert secret containing Encryption configuration is generated
			client, err := helpers.TestEnv.GetK8sClientWithScheme(clientScheme)
			Expect(err).To(BeNil())

			gotSecret := testEncryptionSecretObj()
			err = client.Get(
				ctx,
				ctrlclient.ObjectKeyFromObject(gotSecret),
				gotSecret)
			Expect(err).To(BeNil())
			assert.Equal(
				GinkgoT(),
				testEncryptionConfigSecretData,
				string(gotSecret.Data[SecretKeyForEtcdEncryption]))
		})
	})
})

func testEncryptionSecretObj() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultEncryptionSecretName(request.ClusterName),
			Namespace: request.Namespace,
		},
	}
}
