// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package encryptionatrest

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiserverv1 "k8s.io/apiserver/pkg/apis/config/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	k8sClientUtil "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
)

const (
	// VariableName is the external patch variable name.
	VariableName                        = "encryptionAtRest"
	SecretKeyForEtcdEncryption          = "config"
	defaultEncryptionSecretNameTemplate = "%s-encryption-config" //nolint:gosec // Does not contain hard coded credentials.
	encryptionConfigurationOnRemote     = "/etc/kubernetes/pki/encryptionconfig.yaml"
	apiServerEncryptionConfigArg        = "encryption-provider-config"
)

type encryptionPatchHandler struct {
	client            ctrlclient.Client
	keyGenerator      TokenGenerator
	variableName      string
	variableFieldPath []string
}

func NewPatch(client ctrlclient.Client, keyGenerator TokenGenerator) *encryptionPatchHandler {
	return &encryptionPatchHandler{
		client:            client,
		keyGenerator:      keyGenerator,
		variableName:      v1alpha1.ClusterConfigVariableName,
		variableFieldPath: []string{VariableName},
	}
}

func (h *encryptionPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx, "holderRef", holderRef)

	encryptionVariable, err := variables.Get[v1alpha1.EncryptionAtRest](
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
			cluster, err := clusterGetter(ctx)
			if err != nil {
				log.Error(err, "failed to get cluster from encryption mutation handler")
				return err
			}

			found, err := h.defaultEncryptionSecretExists(ctx, cluster)
			if err != nil {
				log.WithValues(
					"defaultEncryptionSecret", defaultEncryptionSecretName(cluster.Name),
				).Error(err, "failed to find default encryption configuration secret")
				return err
			}

			// we do not rotate or override the secret keys for encryption configuration
			if !found {
				encryptionConfig, err := h.generateEncryptionConfiguration(
					encryptionVariable.Providers,
				)
				if err != nil {
					return err
				}
				if err := h.createEncryptionConfigurationSecret(ctx, encryptionConfig, cluster); err != nil {
					return err
				}
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding encryption configuration files and API server extra args in control plane kubeadm config spec")

			// Create kubeadm config file for encryption config
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				generateEncryptionCredentialsFile(cluster))

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

func generateEncryptionCredentialsFile(cluster *clusterv1.Cluster) cabpkv1.File {
	secretName := defaultEncryptionSecretName(cluster.Name)
	return cabpkv1.File{
		Path: encryptionConfigurationOnRemote,
		ContentFrom: &cabpkv1.FileSource{
			Secret: cabpkv1.SecretFileSource{
				Name: secretName,
				Key:  SecretKeyForEtcdEncryption,
			},
		},
		Permissions: "0640",
	}
}

func (h *encryptionPatchHandler) generateEncryptionConfiguration(
	providers []v1alpha1.EncryptionProviders,
) (*apiserverv1.EncryptionConfiguration, error) {
	resourceConfigs := []apiserverv1.ResourceConfiguration{}
	for _, encProvider := range providers {
		provider := encProvider
		resourceConfig, err := defaultEncryptionConfiguration(
			&provider,
			h.keyGenerator,
		)
		if err != nil {
			return nil, err
		}
		resourceConfigs = append(resourceConfigs, *resourceConfig)
	}
	// We only support encryption for "secrets" and "configmaps" using "aescbc" provider.

	return &apiserverv1.EncryptionConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiserverv1.SchemeGroupVersion.String(),
			Kind:       "EncryptionConfiguration",
		},
		Resources: resourceConfigs,
	}, nil
}

func (h *encryptionPatchHandler) defaultEncryptionSecretExists(
	ctx context.Context,
	cluster *clusterv1.Cluster,
) (bool, error) {
	secretName := defaultEncryptionSecretName(cluster.Name)
	existingSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cluster.Namespace,
		},
	}
	err := h.client.Get(ctx, ctrlclient.ObjectKeyFromObject(existingSecret), existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (h *encryptionPatchHandler) createEncryptionConfigurationSecret(
	ctx context.Context,
	encryptionConfig *apiserverv1.EncryptionConfiguration,
	cluster *clusterv1.Cluster,
) error {
	dataYaml, err := yaml.Marshal(encryptionConfig)
	if err != nil {
		return fmt.Errorf("unable to marshal encryption configuration to YAML: %w", err)
	}

	secretData := map[string]string{
		SecretKeyForEtcdEncryption: strings.TrimSpace(string(dataYaml)),
	}
	secretName := defaultEncryptionSecretName(cluster.Name)
	encryptionConfigSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: cluster.Namespace,
			Labels:    utils.NewLabels(utils.WithMove(), utils.WithClusterName(cluster.Name)),
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}

	if err = controllerutil.SetOwnerReference(cluster, encryptionConfigSecret, h.client.Scheme()); err != nil {
		return fmt.Errorf(
			"failed to set owner reference on encryption configuration secret: %w",
			err,
		)
	}

	// We only support creating encryption config in BeforeClusterCreate hook and ensure that the keys are immutable.
	if err := k8sClientUtil.Create(ctx, h.client, encryptionConfigSecret); err != nil {
		return fmt.Errorf("failed to create encryption configuration secret: %w", err)
	}
	return nil
}

// We only support encryption for "secrets" and "configmaps".
func defaultEncryptionConfiguration(
	providers *v1alpha1.EncryptionProviders,
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
