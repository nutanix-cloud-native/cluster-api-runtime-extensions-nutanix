// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/mirrors"
)

type imageRegistriesPatchHandler struct {
	client ctrlclient.Client

	variableName      string
	variableFieldPath []string
}

var ErrCredentialsNotFound = errors.New("registry credentials not found")

func NewPatch(
	cl ctrlclient.Client,
) *imageRegistriesPatchHandler {
	return newImageRegistriesPatchHandler(
		cl,
		clusterconfig.MetaVariableName,
		imageregistries.VariableName,
	)
}

func newImageRegistriesPatchHandler(
	cl ctrlclient.Client,
	variableName string,
	variableFieldPath ...string,
) *imageRegistriesPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &imageRegistriesPatchHandler{
		client:            cl,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *imageRegistriesPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	imageRegistries, imageRegistriesFound, err := variables.Get[v1alpha1.ImageRegistries](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}

	// add credentials for global image registry mirror
	globalMirror, globalMirrorFound, err := variables.Get[v1alpha1.GlobalImageRegistryMirror](
		vars,
		h.variableName,
		mirrors.GlobalMirrorVariableName,
	)
	if err != nil {
		return err
	}

	if !imageRegistriesFound && !globalMirrorFound {
		log.V(5).Info("Image Registry Credentials variable not defined")
		return nil
	}

	registriesWithOptionalCredentials := make([]providerConfig, 0, len(imageRegistries))
	for _, imageRegistry := range imageRegistries {
		registryWithOptionalCredentials, generateErr := registryWithOptionalCredentialsFromImageRegistryCredentials(
			ctx,
			h.client,
			imageRegistry,
			obj,
		)
		if generateErr != nil {
			return generateErr
		}

		registriesWithOptionalCredentials = append(
			registriesWithOptionalCredentials,
			registryWithOptionalCredentials,
		)
	}

	if globalMirrorFound {
		mirrorCredentials, generateErr := mirrorConfigFromGlobalImageRegistryMirror(
			ctx,
			h.client,
			globalMirror,
			obj,
		)
		if generateErr != nil {
			return err
		}
		registriesWithOptionalCredentials = append(
			registriesWithOptionalCredentials,
			mirrorCredentials,
		)
	}

	needCredentials, err := needImageRegistryCredentialsConfiguration(registriesWithOptionalCredentials)
	if err != nil {
		return err
	}
	if !needCredentials {
		log.V(5).Info("Only Global Registry Mirror is defined but credentials are not needed")
		return nil
	}

	files, commands, generateErr := generateFilesAndCommands(
		registriesWithOptionalCredentials,
		clusterKey.Name,
	)
	if generateErr != nil {
		return generateErr
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding files to control plane kubeadm config spec")
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				files...,
			)

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding PreKubeadmCommands to control plane kubeadm config spec")
			obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands,
				commands...,
			)

			generateErr = createSecretIfNeeded(ctx, h.client, registriesWithOptionalCredentials, clusterKey)
			if generateErr != nil {
				return generateErr
			}

			initConfiguration := obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration
			if initConfiguration == nil {
				initConfiguration = &bootstrapv1.InitConfiguration{}
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.InitConfiguration = initConfiguration
			if initConfiguration.NodeRegistration.KubeletExtraArgs == nil {
				initConfiguration.NodeRegistration.KubeletExtraArgs = map[string]string{}
			}
			addImageCredentialProviderArgs(initConfiguration.NodeRegistration.KubeletExtraArgs)

			joinConfiguration := obj.Spec.Template.Spec.KubeadmConfigSpec.JoinConfiguration
			if joinConfiguration == nil {
				joinConfiguration = &bootstrapv1.JoinConfiguration{}
			}
			obj.Spec.Template.Spec.KubeadmConfigSpec.JoinConfiguration = joinConfiguration
			if joinConfiguration.NodeRegistration.KubeletExtraArgs == nil {
				joinConfiguration.NodeRegistration.KubeletExtraArgs = map[string]string{}
			}
			addImageCredentialProviderArgs(joinConfiguration.NodeRegistration.KubeletExtraArgs)
			return nil
		}); err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.WorkersKubeadmConfigTemplateSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding files to worker node kubeadm config template")
			obj.Spec.Template.Spec.Files = append(obj.Spec.Template.Spec.Files, files...)

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding PreKubeadmCommands to worker node kubeadm config template")
			obj.Spec.Template.Spec.PreKubeadmCommands = append(obj.Spec.Template.Spec.PreKubeadmCommands, commands...)

			generateErr := createSecretIfNeeded(ctx, h.client, registriesWithOptionalCredentials, clusterKey)
			if generateErr != nil {
				return generateErr
			}

			joinConfiguration := obj.Spec.Template.Spec.JoinConfiguration
			if joinConfiguration == nil {
				joinConfiguration = &bootstrapv1.JoinConfiguration{}
			}
			obj.Spec.Template.Spec.JoinConfiguration = joinConfiguration
			if joinConfiguration.NodeRegistration.KubeletExtraArgs == nil {
				joinConfiguration.NodeRegistration.KubeletExtraArgs = map[string]string{}
			}
			addImageCredentialProviderArgs(joinConfiguration.NodeRegistration.KubeletExtraArgs)

			return nil
		}); err != nil {
		return err
	}

	return nil
}

func registryWithOptionalCredentialsFromImageRegistryCredentials(
	ctx context.Context,
	c ctrlclient.Client,
	imageRegistry v1alpha1.ImageRegistry,
	obj ctrlclient.Object,
) (providerConfig, error) {
	registryWithOptionalCredentials := providerConfig{
		URL: imageRegistry.URL,
	}
	secret, err := secretForImageRegistryCredentials(
		ctx,
		c,
		imageRegistry.Credentials,
		obj.GetNamespace(),
	)
	if err != nil {
		return providerConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Image Registry variable: %w",
			obj.GetNamespace(),
			imageRegistry.Credentials.SecretRef.Name,
			err,
		)
	}

	if secret != nil {
		registryWithOptionalCredentials.Username = string(secret.Data["username"])
		registryWithOptionalCredentials.Password = string(secret.Data["password"])
	}

	return registryWithOptionalCredentials, nil
}

func mirrorConfigFromGlobalImageRegistryMirror(
	ctx context.Context,
	c ctrlclient.Client,
	mirror v1alpha1.GlobalImageRegistryMirror,
	obj ctrlclient.Object,
) (providerConfig, error) {
	mirrorCredentials := providerConfig{
		URL:    mirror.URL,
		Mirror: true,
	}
	secret, err := secretForImageRegistryCredentials(
		ctx,
		c,
		mirror.Credentials,
		obj.GetNamespace(),
	)
	if err != nil {
		return providerConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Global Image Registry Mirror variable: %w",
			obj.GetNamespace(),
			mirror.Credentials.SecretRef.Name,
			err,
		)
	}

	if secret != nil {
		mirrorCredentials.Username = string(secret.Data["username"])
		mirrorCredentials.Password = string(secret.Data["password"])
	}

	return mirrorCredentials, nil
}

func generateFilesAndCommands(
	registriesWithOptionalCredentials []providerConfig,
	clusterName string,
) ([]bootstrapv1.File, []string, error) {
	files, commands, err := templateFilesAndCommandsForInstallKubeletCredentialProviders()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"error generating install files and commands for Image Registry Credentials variable: %w",
			err,
		)
	}
	imageCredentialProviderConfigFiles, err := templateFilesForImageCredentialProviderConfigs(
		registriesWithOptionalCredentials,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"error generating files for Image Registry Credentials variable: %w",
			err,
		)
	}
	files = append(files, imageCredentialProviderConfigFiles...)
	files = append(
		files,
		generateCredentialsSecretFile(registriesWithOptionalCredentials, clusterName)...)
	return files, commands, err
}

func createSecretIfNeeded(
	ctx context.Context,
	c ctrlclient.Client,
	registriesWithOptionalCredentials []providerConfig,
	clusterKey ctrlclient.ObjectKey,
) error {
	credentialsSecret, err := generateCredentialsSecret(
		registriesWithOptionalCredentials,
		clusterKey.Name,
		clusterKey.Namespace,
	)
	if err != nil {
		return fmt.Errorf(
			"error generating credentials Secret for Image Registry Credentials variable: %w",
			err,
		)
	}
	if credentialsSecret != nil {
		if err := client.ServerSideApply(ctx, c, credentialsSecret); err != nil {
			return fmt.Errorf("failed to apply Image Registry Credentials Secret: %w", err)
		}
	}

	return nil
}

// secretForImageRegistryCredentials returns the Secret for the given ImageRegistryCredentials.
// Returns nil if the secret field is empty.
func secretForImageRegistryCredentials(
	ctx context.Context,
	c ctrlclient.Reader,
	credentials *v1alpha1.RegistryCredentials,
	objectNamespace string,
) (*corev1.Secret, error) {
	if credentials == nil || credentials.SecretRef == nil {
		return nil, nil
	}

	key := ctrlclient.ObjectKey{
		Name:      credentials.SecretRef.Name,
		Namespace: objectNamespace,
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, key, secret)
	return secret, err
}

// This handler reads input from two user provided variables: globalImageRegistryMirror and imageRegistries.
// We expect if imageRegistries is set it will either have static credentials
// or be for a registry where the credential plugin returns the credentials, ie ECR, GCR, ACR, etc,
// and if that is not the case we assume the users missed setting static credentials and return an error.
// However, in addition to passing credentials with the globalImageRegistryMirror variable,
// it can also be used to only set Containerd mirror configuration,
// in that case it valid for static credentials to not be set and will return false, no error
// and this handler will skip generating any credential plugin related configuration.
func needImageRegistryCredentialsConfiguration(configs []providerConfig) (bool, error) {
	for _, config := range configs {
		requiresStaticCredentials, err := config.requiresStaticCredentials()
		if err != nil {
			return false,
				fmt.Errorf("error determining if Image Registry is a supported provider: %w", err)
		}
		// verify the credentials are actually set if the plugin requires static credentials
		if config.isCredentialsEmpty() && requiresStaticCredentials {
			// not setting credentials for a mirror is valid
			// but if it's the only configuration then return false here and exit the handler early
			if config.Mirror {
				if len(configs) == 1 {
					return false, nil
				}
			} else {
				return false, fmt.Errorf("invalid image registry: %s: %w", config.URL, ErrCredentialsNotFound)
			}
		}
	}

	return true, nil
}
