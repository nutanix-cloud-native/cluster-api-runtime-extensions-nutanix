// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiserverv1 "k8s.io/apiserver/pkg/apis/config/v1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName                        = "encryption"
	SecretKeyForEtcdEncryption          = "config"
	defaultEncryptionSecretNameTemplate = "%s-encryption-config" //nolint:gosec // Does not contain hard coded credentials.
	encryptionConfigurationOnRemote     = "/etc/kubernetes/encryptionconfig.yaml"
	apiServerEncryptionConfigArg        = "encryption-provider-config"
)

type Config struct {
	Client                ctrlclient.Client
	AESSecretKeyGenerator TokenGenerator
}

type encryptionPatchHandler struct {
	config            *Config
	variableName      string
	variableFieldPath []string
}

func NewPatch(config *Config) *encryptionPatchHandler {
	return newEncryptionPatchHandler(
		config,
		clusterconfig.MetaVariableName,
		VariableName)
}

func newEncryptionPatchHandler(
	config *Config,
	variableName string,
	variableFieldPath ...string,
) *encryptionPatchHandler {
	return &encryptionPatchHandler{
		config:            config,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *encryptionPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx, "holderRef", holderRef)

	encryptionVariable, err := variables.Get[carenv1.Encryption](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("encryption variable not defined")
			return nil
		}
		return err
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		encryptionVariable,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("setting encryption in control plane kubeadm config template")
			encConfig, err := h.generateEncryptionConfiguration(encryptionVariable.Providers)
			if err != nil {
				return err
			}
			secretName, err := h.CreateEncryptionConfigurationSecret(ctx, encConfig, clusterKey)
			if err != nil {
				return err
			}
			// Create kubadm config file for encryption config
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				generateEncryptionCredentialsFile(secretName))

			// set APIServer args for encryption config
			if obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration == nil {
				obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration = &cabpkv1.ClusterConfiguration{}
			}
			apiServer := &obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer
			if apiServer.ExtraArgs == nil {
				apiServer.ExtraArgs = make(map[string]string, 1)
			}
			apiServer.ExtraArgs[apiServerEncryptionConfigArg] = encryptionConfigurationOnRemote

			return nil
		})
}

func generateEncryptionCredentialsFile(secretName string) cabpkv1.File {
	return cabpkv1.File{
		Path: encryptionConfigurationOnRemote,
		ContentFrom: &cabpkv1.FileSource{
			Secret: cabpkv1.SecretFileSource{
				Name: secretName,
				Key:  SecretKeyForEtcdEncryption,
			},
		},
		Permissions: "0600",
	}
}

func (h *encryptionPatchHandler) generateEncryptionConfiguration(
	providers *carenv1.EncryptionProviders,
) (*apiserverv1.EncryptionConfiguration, error) {
	// We only support encryption for "secrets" and "configmaps" using "aescbc" provider.
	resourceConfig, err := encryptionConfigForSecretsAndConfigMaps(
		providers,
		h.config.AESSecretKeyGenerator,
	)
	if err != nil {
		return nil, err
	}
	return &apiserverv1.EncryptionConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1.SchemeGroupVersion.String(),
			Kind:       "EncryptionConfiguration",
		},
		Resources: []apiserverv1.ResourceConfiguration{
			*resourceConfig,
		},
	}, nil
}

func (h *encryptionPatchHandler) CreateEncryptionConfigurationSecret(
	ctx context.Context,
	encryptionConfig *apiserverv1.EncryptionConfiguration,
	clusterKey ctrlclient.ObjectKey,
) (string, error) {
	dataYaml, err := yaml.Marshal(encryptionConfig)
	if err != nil {
		return "", fmt.Errorf("unable to marshal encryption configuration to YAML: %w", err)
	}

	secretData := map[string]string{
		SecretKeyForEtcdEncryption: strings.TrimSpace(string(dataYaml)),
	}
	secretName := defaultEncryptionSecretName(clusterKey.Name)
	encryptionConfigSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: clusterKey.Namespace,
			Labels:    utils.NewLabels(utils.WithMove(), utils.WithClusterName(clusterKey.Name)),
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}

	// We only support creating encryption config in BeforeClusterCreate hook and ensure that the keys are immutable.
	if err := client.Create(ctx, h.config.Client, encryptionConfigSecret); err != nil {
		return "", fmt.Errorf("failed to create encryption configuration secret: %w", err)
	}
	return secretName, nil
}

// We only support encryption for "secrets" and "configmaps".
func encryptionConfigForSecretsAndConfigMaps(
	providers *carenv1.EncryptionProviders,
	secretGenerator TokenGenerator,
) (*apiserverv1.ResourceConfiguration, error) {
	providerConfig := apiserverv1.ProviderConfiguration{}
	// We only support "aescbc", "secretbox" for now.
	// "aesgcm" is another AESConfiguration. "aesgcm" requires secret key rotation before 200k write calls.
	// "aesgcm" should not be supported until secret key's rotation is implemented.
	if providers.AESCBC != nil {
		token, err := secretGenerator()
		if err != nil {
			return nil, fmt.Errorf(
				"could not create random encryption token for aescbc provider: %w",
				err,
			)
		}
		providerConfig.AESCBC = &apiserverv1.AESConfiguration{
			Keys: []apiserverv1.Key{
				{
					Name:   "key1", // we only support one key during cluster creation.
					Secret: base64.StdEncoding.EncodeToString(token),
				},
			},
		}
	}
	if providers.Secretbox != nil {
		token, err := secretGenerator()
		if err != nil {
			return nil, fmt.Errorf(
				"could not create random encryption token for secretbox provider: %w",
				err,
			)
		}
		providerConfig.Secretbox = &apiserverv1.SecretboxConfiguration{
			Keys: []apiserverv1.Key{
				{
					Name:   "key1", // we only support one key during cluster creation.
					Secret: base64.StdEncoding.EncodeToString(token),
				},
			},
		}
	}

	return &apiserverv1.ResourceConfiguration{
		Resources: []string{"secrets", "configmaps"},
		Providers: []apiserverv1.ProviderConfiguration{
			providerConfig,
		},
	}, nil
}

func defaultEncryptionSecretName(clusterName string) string {
	return fmt.Sprintf(defaultEncryptionSecretNameTemplate, clusterName)
}
