// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mirrors

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	registryutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/registry/utils"
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
	_ ctrlclient.ObjectKey,
	clusterGetter mutation.ClusterGetter,
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

	_, registryAddonErr := variables.Get[v1alpha1.RegistryAddon](
		vars,
		v1alpha1.ClusterConfigVariableName,
		[]string{"addons", v1alpha1.RegistryAddonVariableName}...)

	switch {
	case variables.IsNotFoundError(imageRegistriesErr) &&
		variables.IsNotFoundError(globalMirrorErr) &&
		variables.IsNotFoundError(registryAddonErr):
		log.V(5).
			Info("Image Registry Credentials and Global Registry Mirror and Registry Addon variable not defined")
		return nil
	case imageRegistriesErr != nil && !variables.IsNotFoundError(imageRegistriesErr):
		return imageRegistriesErr
	case globalMirrorErr != nil && !variables.IsNotFoundError(globalMirrorErr):
		return globalMirrorErr
	case registryAddonErr != nil && !variables.IsNotFoundError(registryAddonErr):
		return registryAddonErr
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
	if registryAddonErr == nil {
		cluster, err := clusterGetter(ctx)
		if err != nil {
			log.Error(
				err,
				"failed to get cluster from Global Mirror mutation handler",
			)
			return err
		}

		registryConfig, err := containerdConfigFromRegistryAddon(
			ctx,
			h.client,
			cluster,
		)
		if err != nil {
			return err
		}
		registriesWithOptionalCA = append(registriesWithOptionalCA, registryConfig)
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

	if secretHasCACert(secret) {
		configWithOptionalCACert.CASecretName = secret.Name
		configWithOptionalCACert.CACert = string(secret.Data[secretKeyForCACert])
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

	if secretHasCACert(secret) {
		configWithOptionalCACert.CASecretName = secret.Name
		configWithOptionalCACert.CACert = string(secret.Data[secretKeyForCACert])
	}

	return configWithOptionalCACert, nil
}

func containerdConfigFromRegistryAddon(
	ctx context.Context,
	c ctrlclient.Client,
	cluster *clusterv1.Cluster,
) (containerdConfig, error) {
	serviceIP, err := registryutils.ServiceIPForCluster(cluster)
	if err != nil {
		return containerdConfig{}, fmt.Errorf("error getting service IP for the registry addon: %w", err)
	}
	secret, err := handlersutils.SecretForClusterRegistryAddonCA(ctx, c, cluster)
	if err != nil {
		return containerdConfig{}, fmt.Errorf("error getting CA secret for registry addon: %w", err)
	}
	if !secretHasCACert(secret) {
		return containerdConfig{}, errors.New("CA certificate not found in the secret")
	}
	config := containerdConfig{
		URL:          fmt.Sprintf("https://%s", serviceIP),
		Mirror:       true,
		CASecretName: secret.Name,
		CACert:       string(secret.Data[secretKeyForCACert]),
	}

	return config, nil
}

func generateFiles(
	registriesWithOptionalCA []containerdConfig,
) ([]bootstrapv1.File, error) {
	var files []bootstrapv1.File
	// generate default registry mirror file
	containerdHostsFile, err := generateContainerdDefaultHostsFile(registriesWithOptionalCA)
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

func secretHasCACert(secret *corev1.Secret) bool {
	if secret == nil {
		return false
	}

	_, ok := secret.Data[secretKeyForCACert]
	return ok
}
