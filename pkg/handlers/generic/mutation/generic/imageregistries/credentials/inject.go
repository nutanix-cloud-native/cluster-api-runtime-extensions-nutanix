// Copyright 2023 Nutanix. All rights reserved.
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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/k8s/client"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
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
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.ImageRegistriesVariableName,
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
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	imageRegistries, imageRegistriesErr := variables.Get[[]v1alpha1.ImageRegistry](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)

	// add credentials for global image registry mirror
	globalMirror, globalMirrorErr := variables.Get[v1alpha1.GlobalImageRegistryMirror](
		vars,
		h.variableName,
		v1alpha1.GlobalMirrorVariableName,
	)

	switch {
	case variables.IsNotFoundError(imageRegistriesErr) && variables.IsNotFoundError(globalMirrorErr):
		log.V(5).Info("Image Registry Credentials and Global Registry Mirror variable not defined")
		return nil
	case imageRegistriesErr != nil && !variables.IsNotFoundError(imageRegistriesErr):
		return imageRegistriesErr
	case globalMirrorErr != nil && !variables.IsNotFoundError(globalMirrorErr):
		return globalMirrorErr
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

	if globalMirrorErr == nil {
		mirrorCredentials, generateErr := mirrorWithOptionalCredentialsFromGlobalImageRegistryMirror(
			ctx,
			h.client,
			globalMirror,
			obj,
		)
		if generateErr != nil {
			return generateErr
		}
		registriesWithOptionalCredentials = append(
			registriesWithOptionalCredentials,
			mirrorCredentials,
		)
	}

	registriesThatNeedConfiguration, err := providerConfigsThatNeedConfiguration(
		registriesWithOptionalCredentials,
	)
	if err != nil {
		return err
	}
	if len(registriesThatNeedConfiguration) == 0 {
		log.V(5).Info("Image registry credentials are not needed")
		return nil
	}

	files, commands, generateErr := generateFilesAndCommands(
		registriesThatNeedConfiguration,
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

			cluster, err := clusterGetter(ctx)
			if err != nil {
				log.Error(
					err,
					"failed to get cluster from Image Registry Credentials mutation handler",
				)
				return err
			}

			err = ensureOwnerReferenceOnCredentialsSecrets(ctx, h.client, imageRegistries, globalMirror, cluster)
			if err != nil {
				return err
			}

			err = createSecretIfNeeded(ctx, h.client, registriesThatNeedConfiguration, cluster)
			if err != nil {
				return err
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

			cluster, err := clusterGetter(ctx)
			if err != nil {
				log.Error(
					err,
					"failed to get cluster from Image Registry Credentials mutation handler",
				)
				return err
			}

			err = ensureOwnerReferenceOnCredentialsSecrets(ctx, h.client, imageRegistries, globalMirror, cluster)
			if err != nil {
				return err
			}

			err = createSecretIfNeeded(ctx, h.client, registriesThatNeedConfiguration, cluster)
			if err != nil {
				return err
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

func ensureOwnerReferenceOnCredentialsSecrets(
	ctx context.Context,
	c ctrlclient.Client,
	imageRegistries []v1alpha1.ImageRegistry,
	globalMirror v1alpha1.GlobalImageRegistryMirror,
	cluster *clusterv1.Cluster,
) error {
	var credentials []*v1alpha1.RegistryCredentials
	for _, imageRegistry := range imageRegistries {
		if imageRegistry.Credentials != nil {
			credentials = append(credentials, imageRegistry.Credentials)
		}
	}
	if globalMirror.Credentials != nil {
		credentials = append(credentials, globalMirror.Credentials)
	}

	for _, credential := range credentials {
		if secretName := handlersutils.SecretNameForImageRegistryCredentials(credential); secretName != "" {
			// Ensure the Secret is owned by the Cluster so it is correctly moved and deleted with the Cluster.
			// This code assumes that Secret exists and that was validated before calling this function.
			err := handlersutils.EnsureClusterOwnerReferenceForObject(
				ctx,
				c,
				corev1.TypedLocalObjectReference{
					Kind: "Secret",
					Name: secretName,
				},
				cluster,
			)
			if err != nil {
				return fmt.Errorf(
					"error updating owner references on image registry Secret: %w",
					err,
				)
			}
		}
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
	secret, err := handlersutils.SecretForImageRegistryCredentials(
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
		registryWithOptionalCredentials.HasCACert = secretHasCACert(secret)
	}

	return registryWithOptionalCredentials, nil
}

func mirrorWithOptionalCredentialsFromGlobalImageRegistryMirror(
	ctx context.Context,
	c ctrlclient.Client,
	mirror v1alpha1.GlobalImageRegistryMirror,
	obj ctrlclient.Object,
) (providerConfig, error) {
	mirrorCredentials := providerConfig{
		URL:    mirror.URL,
		Mirror: true,
	}
	secret, err := handlersutils.SecretForImageRegistryCredentials(
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
		mirrorCredentials.HasCACert = secretHasCACert(secret)
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

	credentialSecretFile := generateCredentialsSecretFile(
		registriesWithOptionalCredentials,
		clusterName,
	)
	if credentialSecretFile != nil {
		files = append(files, *credentialSecretFile)
	}

	return files, commands, err
}

func createSecretIfNeeded(
	ctx context.Context,
	c ctrlclient.Client,
	registriesWithOptionalCredentials []providerConfig,
	cluster *clusterv1.Cluster,
) error {
	credentialsSecret, err := generateCredentialsSecret(
		registriesWithOptionalCredentials,
		cluster.Name,
		cluster.Namespace,
	)
	if err != nil {
		return fmt.Errorf(
			"error generating credentials Secret for Image Registry Credentials variable: %w",
			err,
		)
	}
	if credentialsSecret != nil {
		if err = controllerutil.SetOwnerReference(cluster, credentialsSecret, c.Scheme()); err != nil {
			return fmt.Errorf(
				"failed to set owner reference on Image Registry Credentials Secret: %w",
				err,
			)
		}
		if err := client.ServerSideApply(ctx, c, credentialsSecret, client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply Image Registry Credentials Secret: %w", err)
		}
	}

	return nil
}

// This handler reads input from two user provided variables: globalImageRegistryMirror and imageRegistries.
// We expect if imageRegistries is set it will either have static credentials
// or be for a registry where the credential plugin returns the credentials, ie ECR, GCR, ACR, etc,
// or have no credentials set but to contain a CA cert,
// and if that is not the case we assume the users missed setting static credentials and return an error.
// However, in addition to passing credentials with the globalImageRegistryMirror variable,
// it can also be used to only set Containerd mirror configuration,
// in which case it is valid for static credentials to not be set and will be skipped, no error
// and this handler will skip generating any credential plugin related configuration.
func providerConfigsThatNeedConfiguration(configs []providerConfig) ([]providerConfig, error) {
	var needConfiguration []providerConfig //nolint:prealloc // We don't know the size of the slice yet.
	for _, config := range configs {
		requiresStaticCredentials, err := config.requiresStaticCredentials()
		if err != nil {
			return nil,
				fmt.Errorf("error determining if Image Registry is a supported provider: %w", err)
		}
		// verify the credentials are actually set if the plugin requires static credentials
		if config.isCredentialsEmpty() && requiresStaticCredentials {
			if config.Mirror || config.HasCACert {
				// not setting credentials for a mirror is valid, but won't need any configuration
				// not setting credentials for a registry with a CA cert is valid, but won't need any configuration
				continue
			}
			return nil, fmt.Errorf(
				"invalid image registry: %s: %w",
				config.URL,
				ErrCredentialsNotFound,
			)
		}
		needConfiguration = append(needConfiguration, config)
	}

	return needConfiguration, nil
}
