// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package credentials

import (
	"context"
	_ "embed"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	cabpkv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/k8s/client"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation/imageregistries"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "ImageRegistryCredentialsPatch"
)

type imageRegistriesPatchHandler struct {
	decoder runtime.Decoder
	client  ctrlclient.Client

	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &imageRegistriesPatchHandler{}
	_ mutation.GeneratePatches = &imageRegistriesPatchHandler{}
	_ mutation.MetaMutater     = &imageRegistriesPatchHandler{}
)

func NewPatch(
	cl ctrlclient.Client,
) *imageRegistriesPatchHandler {
	return newImageRegistriesPatchHandler(cl, variableName)
}

func NewMetaPatch(
	cl ctrlclient.Client,
) *imageRegistriesPatchHandler {
	return newImageRegistriesPatchHandler(cl, clusterconfig.MetaVariableName, imageregistries.VariableName, variableName)
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
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
		client:            cl,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *imageRegistriesPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *imageRegistriesPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey ctrlclient.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	imageRegistryCredentials, found, err := variables.Get[v1alpha1.ImageRegistryCredentials](
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
	if len(imageRegistryCredentials) > 1 {
		return fmt.Errorf("multiple Image Registry Credentials are not supported at this time")
	}

	credentials := imageRegistryCredentials[0]

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		credentials,
	)

	if err = patches.Generate(
		obj, vars, &holderRef, selectors.ControlPlane(), log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			registryWithOptionalCredentials, generateErr :=
				registryWithOptionalCredentialsFromImageRegistryCredentials(ctx, h.client, credentials, obj)
			if generateErr != nil {
				return generateErr
			}
			files, commands, generateErr := generateFilesAndCommands(registryWithOptionalCredentials, obj.GetName())
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

			return nil
		}); err != nil {
		return err
	}

	if err = patches.Generate(
		obj, vars, &holderRef, selectors.AllWorkersSelector(), log,
		func(obj *bootstrapv1.KubeadmConfigTemplate) error {
			registryWithOptionalCredentials, generateErr :=
				registryWithOptionalCredentialsFromImageRegistryCredentials(ctx, h.client, credentials, obj)
			if generateErr != nil {
				return generateErr
			}
			files, commands, generateErr := generateFilesAndCommands(registryWithOptionalCredentials, obj.GetName())
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

			return nil
		}); err != nil {
		return err
	}

	return nil
}

func (h *imageRegistriesPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	clusterKey := commonhandlers.ClusterKeyFromReq(req)

	topologymutation.WalkTemplates(
		ctx,
		h.decoder,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			return h.Mutate(ctx, obj, vars, holderRef, clusterKey)
		},
	)
}

func registryWithOptionalCredentialsFromImageRegistryCredentials(
	ctx context.Context,
	c ctrlclient.Client,
	credentials v1alpha1.ImageRegistryCredentialsResource,
	obj ctrlclient.Object,
) (providerInput, error) {
	registryWithOptionalCredentials := providerInput{
		URL: credentials.URL,
	}
	secret, err := secretForImageRegistryCredentials(
		ctx,
		c,
		credentials,
		obj.GetNamespace(),
	)
	if err != nil {
		return providerInput{}, fmt.Errorf(
			"error getting secret %s/%s from Image Registry Credentials variable: %w",
			obj.GetNamespace(),
			credentials.Secret,
			err,
		)
	}

	if secret != nil {
		registryWithOptionalCredentials.Username = string(secret.Data["username"])
		registryWithOptionalCredentials.Password = string(secret.Data["password"])
	}

	return registryWithOptionalCredentials, nil
}

func generateFilesAndCommands(
	registryWithOptionalCredentials providerInput,
	objName string,
) ([]cabpkv1.File, []string, error) {

	files, commands, err := templateFilesAndCommandsForInstallKubeletCredentialProviders()
	if err != nil {
		return nil, nil, fmt.Errorf("error generating insall files and commands for Image Registry Credentials variable: %w", err)
	}
	imageCredentialProviderConfigFiles, err := templateFilesForImageCredentialProviderConfigs(registryWithOptionalCredentials)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating files for Image Registry Credentials variable: %w", err)
	}
	files = append(files, imageCredentialProviderConfigFiles...)
	files = append(files, generateCredentialsSecretFile(registryWithOptionalCredentials, objName)...)

	return files, commands, err
}

func createSecretIfNeeded(
	ctx context.Context,
	c ctrlclient.Client,
	registryWithOptionalCredentials providerInput,
	obj ctrlclient.Object,
	clusterKey ctrlclient.ObjectKey,
) error {
	credentialsSecret, err := generateCredentialsSecret(registryWithOptionalCredentials, clusterKey.Name, obj.GetName(), obj.GetNamespace())
	if err != nil {
		return fmt.Errorf("error generating credentials Secret for Image Registry Credentials variable: %w", err)
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
	credentials v1alpha1.ImageRegistryCredentialsResource,
	objectNamespace string,
) (*corev1.Secret, error) {
	if credentials.Secret == nil {
		return nil, nil
	}

	namespace := objectNamespace
	if credentials.Secret.Namespace != "" {
		namespace = credentials.Secret.Namespace
	}

	key := ctrlclient.ObjectKey{
		Name:      credentials.Secret.Name,
		Namespace: namespace,
	}
	secret := &corev1.Secret{}
	err := c.Get(ctx, key, secret)
	return secret, err
}
