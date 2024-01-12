// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
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
)

type imageRegistriesPatchHandler struct {
	client ctrlclient.Client

	variableName      string
	variableFieldPath []string
}

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

	imageRegistries, found, err := variables.Get[v1alpha1.ImageRegistries](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("Image Registry Credentials variable not defined")
		return nil
	}

	// TODO: Add support for multiple registries.
	if len(imageRegistries) > 1 {
		return fmt.Errorf("multiple Image Registry are not supported at this time")
	}

	imageRegistry := imageRegistries[0]

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		imageRegistry,
	)

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			registryWithOptionalCredentials, generateErr := registryWithOptionalCredentialsFromImageRegistryCredentials(
				ctx, h.client, imageRegistry, obj,
			)
			if generateErr != nil {
				return generateErr
			}
			files, commands, generateErr := generateFilesAndCommands(registryWithOptionalCredentials, imageRegistry, obj.GetName())
			if generateErr != nil {
				return generateErr
			}

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

			generateErr = createSecretIfNeeded(ctx, h.client, registryWithOptionalCredentials, obj, clusterKey)
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
			registryWithOptionalCredentials, generateErr := registryWithOptionalCredentialsFromImageRegistryCredentials(
				ctx, h.client, imageRegistry, obj,
			)
			if generateErr != nil {
				return generateErr
			}
			files, commands, generateErr := generateFilesAndCommands(registryWithOptionalCredentials, imageRegistry, obj.GetName())
			if generateErr != nil {
				return generateErr
			}

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

			generateErr = createSecretIfNeeded(ctx, h.client, registryWithOptionalCredentials, obj, clusterKey)
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
		imageRegistry,
		obj.GetNamespace(),
	)
	if err != nil {
		return providerConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Image Registry variable: %w",
			obj.GetNamespace(),
			imageRegistry.CredentialsSecret,
			err,
		)
	}

	if secret != nil {
		registryWithOptionalCredentials.Username = string(secret.Data["username"])
		registryWithOptionalCredentials.Password = string(secret.Data["password"])
		registryWithOptionalCredentials.CACert = string(secret.Data[secretKeyForMirrorCACert])
	}

	return registryWithOptionalCredentials, nil
}

func generateFilesAndCommands(
	registryWithOptionalCredentials providerConfig,
	imageRegistry v1alpha1.ImageRegistry,
	objName string,
) ([]bootstrapv1.File, []string, error) {
	files, commands, err := templateFilesAndCommandsForInstallKubeletCredentialProviders()
	if err != nil {
		return nil, nil, fmt.Errorf(
			"error generating install files and commands for Image Registry Credentials variable: %w",
			err,
		)
	}
	imageCredentialProviderConfigFiles, err := templateFilesForImageCredentialProviderConfigs(
		registryWithOptionalCredentials,
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
		generateCredentialsSecretFile(registryWithOptionalCredentials, objName)...)

	// Generate default registry mirror file
	mirrorHostFiles, err := generateDefaultRegistryMirrorFile(registryWithOptionalCredentials)
	if err != nil {
		return nil, nil, err
	}
	files = append(
		files,
		mirrorHostFiles...,
	)
	// generate CA certificate file for registry mirror
	files = append(files, generateMirrorCACertFile(registryWithOptionalCredentials, imageRegistry)...)

	return files, commands, err
}

func createSecretIfNeeded(
	ctx context.Context,
	c ctrlclient.Client,
	registryWithOptionalCredentials providerConfig,
	obj ctrlclient.Object,
	clusterKey ctrlclient.ObjectKey,
) error {
	credentialsSecret, err := generateCredentialsSecret(
		registryWithOptionalCredentials,
		clusterKey.Name,
		obj.GetName(),
		obj.GetNamespace(),
	)
	if err != nil {
		return fmt.Errorf(
			"error generating crdentials Secret for Image Registry Credentials variable: %w",
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
	registry v1alpha1.ImageRegistry,
	objectNamespace string,
) (*corev1.Secret, error) {
	if registry.CredentialsSecret == nil {
		return nil, nil
	}

	namespace := objectNamespace
	if registry.CredentialsSecret.Namespace != "" {
		namespace = registry.CredentialsSecret.Namespace
	}

	key := ctrlclient.ObjectKey{
		Name:      registry.CredentialsSecret.Name,
		Namespace: namespace,
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, key, secret)
	return secret, err
}
