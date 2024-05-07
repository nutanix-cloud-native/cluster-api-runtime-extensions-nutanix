// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
    secretbox:
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
	RunSpecs(t, "Encryption configuration mutator suite")
}

var _ = Describe("Generate Encryption configuration patches", func() {
	patchGenerator := func() mutation.GeneratePatches {
		config := &Config{
			Client:                helpers.TestEnv.Client,
			AESSecretKeyGenerator: testTokenGenerator,
		}
		return mutation.NewMetaGeneratePatchesHandler(
			"",
			helpers.TestEnv.Client,
			NewPatch(config)).(mutation.GeneratePatches)
	}

	testDefs := []capitest.PatchTestDef{
		{
			Name: "files added in KubeadmControlPlaneTemplate for Encryption Configuration",
			Vars: []runtimehooksv1.Variable{
				capitest.VariableWithValue(
					clusterconfig.MetaVariableName,
					carenv1.Encryption{
						Providers: []carenv1.EncryptionProvider{
							carenv1.AESCBC,
							carenv1.SecretBox,
						},
					},
					VariableName,
				),
			},
			RequestItem: request.NewKubeadmControlPlaneTemplateRequestItem(""),
			ExpectedPatchMatchers: []capitest.JSONPatchMatcher{
				{
					Operation: "add",
					Path:      "/spec/template/spec/kubeadmConfigSpec/files",
					ValueMatcher: ContainElements(
						SatisfyAll(
							HaveKeyWithValue(
								"path", "/etc/kubernetes/encryptionconfig.yaml",
							),
							HaveKeyWithValue(
								"permissions", "0600",
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
								"encryption-provider-config": "/etc/kubernetes/encryptionconfig.yaml",
							},
						),
					),
				},
			},
		},
	}
	// create test node for each case
	for testIdx := range testDefs {
		tt := testDefs[testIdx]
		It(tt.Name, func(ctx SpecContext) {
			capitest.AssertGeneratePatches(GinkgoT(), patchGenerator, &tt)

			// assert secret containing Encryption configuration is generated
			gotSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultEncryptionSecretName(request.ClusterName),
					Namespace: request.Namespace,
				},
			}
			objName := ctrlclient.ObjectKeyFromObject(gotSecret)
			client, err := helpers.TestEnv.GetK8sClient()
			Expect(err).To(BeNil())

			err = client.Get(ctx, objName, gotSecret)
			Expect(err).To(BeNil())
			GinkgoWriter.Println(string(gotSecret.Data[SecretKeyForEtcdEncryption]))
			assert.Equal(
				GinkgoT(),
				testEncryptionConfigSecretData,
				string(gotSecret.Data[SecretKeyForEtcdEncryption]))
		})
	}
})
