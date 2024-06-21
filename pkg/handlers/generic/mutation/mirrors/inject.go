// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"context"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	handlersutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/utils"
)

type globalMirrorPatchHandler struct {
	client ctrlclient.Client

	variableName      string
	variableFieldPath []string
}

func NewPatch(
	cl ctrlclient.Client,
) *globalMirrorPatchHandler {
	return newGlobalMirrorPatchHandler(
		cl,
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.GlobalMirrorVariableName,
	)
}

func newGlobalMirrorPatchHandler(
	cl ctrlclient.Client,
	variableName string,
	variableFieldPath ...string,
) *globalMirrorPatchHandler {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return &globalMirrorPatchHandler{
		client:            cl,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *globalMirrorPatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
	_ mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	globalMirror, globalMirrorErr := variables.Get[v1alpha1.GlobalImageRegistryMirror](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)

	// add CA certificate for image registries
	imageRegistries, imageRegistriesErr := variables.Get[[]v1alpha1.ImageRegistry](
		vars,
		h.variableName,
		v1alpha1.ImageRegistriesVariableName,
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

	var registriesWithOptionalCA []containerdConfig //nolint:prealloc // We don't know the size of the slice yet.
	if globalMirrorErr == nil {
		registryConfig, err := containerdConfigFromGlobalMirror(
			ctx,
			h.client,
			globalMirror,
			obj,
		)
		if err != nil {
			return err
		}
		registriesWithOptionalCA = append(registriesWithOptionalCA, registryConfig)
	}
	for _, imageRegistry := range imageRegistries {
		registryWithOptionalCredentials, generateErr := containerdConfigFromImageRegistry(
			ctx,
			h.client,
			imageRegistry,
			obj,
		)
		if generateErr != nil {
			return generateErr
		}

		registriesWithOptionalCA = append(
			registriesWithOptionalCA,
			registryWithOptionalCredentials,
		)
	}

	needConfiguration := needContainerdConfiguration(
		registriesWithOptionalCA,
	)
	if !needConfiguration {
		log.V(5).Info("Only Image Registry Configuration is defined but without CA certificates")
		return nil
	}

	files, err := generateFiles(registriesWithOptionalCA)
	if err != nil {
		return err
	}

	if err := patches.MutateIfApplicable(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", ctrlclient.ObjectKeyFromObject(obj),
			).Info("adding global registry mirror files to control plane kubeadm config spec")
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				files...,
			)

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
			).Info("adding global registry mirror files to worker node kubeadm config template")
			obj.Spec.Template.Spec.Files = append(obj.Spec.Template.Spec.Files, files...)

			return nil
		}); err != nil {
		return err
	}

	return nil
}

func containerdConfigFromGlobalMirror(
	ctx context.Context,
	c ctrlclient.Client,
	globalMirror v1alpha1.GlobalImageRegistryMirror,
	obj ctrlclient.Object,
) (containerdConfig, error) {
	configWithOptionalCACert := containerdConfig{
		URL:    globalMirror.URL,
		Mirror: true,
	}
	secret, err := handlersutils.SecretForImageRegistryCredentials(
		ctx,
		c,
		globalMirror.Credentials,
		obj.GetNamespace(),
	)
	if err != nil {
		return containerdConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Global Image Registry Mirror variable: %w",
			obj.GetNamespace(),
			globalMirror.Credentials.SecretRef.Name,
			err,
		)
	}

	if secret != nil {
		configWithOptionalCACert.CASecretName = secret.Name
		configWithOptionalCACert.CACert = string(secret.Data[secretKeyForMirrorCACert])
	}

	return configWithOptionalCACert, nil
}

func containerdConfigFromImageRegistry(
	ctx context.Context,
	c ctrlclient.Client,
	imageRegistry v1alpha1.ImageRegistry,
	obj ctrlclient.Object,
) (containerdConfig, error) {
	configWithOptionalCACert := containerdConfig{
		URL: imageRegistry.URL,
	}
	secret, err := handlersutils.SecretForImageRegistryCredentials(
		ctx,
		c,
		imageRegistry.Credentials,
		obj.GetNamespace(),
	)
	if err != nil {
		return containerdConfig{}, fmt.Errorf(
			"error getting secret %s/%s from Image Registry variable: %w",
			obj.GetNamespace(),
			imageRegistry.Credentials.SecretRef.Name,
			err,
		)
	}

	if secret != nil {
		configWithOptionalCACert.CASecretName = secret.Name
		configWithOptionalCACert.CACert = string(secret.Data[secretKeyForMirrorCACert])
	}

	return configWithOptionalCACert, nil
}

func generateFiles(
	registriesWithOptionalCA []containerdConfig,
) ([]bootstrapv1.File, error) {
	var files []bootstrapv1.File
	// generate default registry mirror file
	containerdHostsFile, err := generateContainerdHostsFile(registriesWithOptionalCA)
	if err != nil {
		return nil, err
	}
	if containerdHostsFile != nil {
		files = append(files, *containerdHostsFile)
	}

	// generate CA certificate file for registry mirror
	mirrorCAFiles, err := generateRegistryCACertFiles(registriesWithOptionalCA)
	if err != nil {
		return nil, err
	}
	files = append(files, mirrorCAFiles...)

	// generate Containerd registry config drop-in file
	registryConfigDropIn := generateContainerdRegistryConfigDropInFile()
	files = append(files, registryConfigDropIn...)

	return files, err
}

// This handler reads input from two user provided variables: globalImageRegistryMirror and imageRegistries.
// The handler will be used to either add configuration for a global mirror or CA certificates for image registries.
func needContainerdConfiguration(configs []containerdConfig) bool {
	for _, config := range configs {
		if config.needContainerdConfiguration() {
			return true
		}
	}

	return false
}
