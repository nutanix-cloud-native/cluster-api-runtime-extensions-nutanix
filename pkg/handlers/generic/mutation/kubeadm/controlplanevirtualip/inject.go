// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"context"
	"fmt"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/controlplanevirtualip/providers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "controlPlaneEndpoint"
)

type Config struct {
	*options.GlobalOptions
}

type ControlPlaneVirtualIP struct {
	variableName      string
	variableFieldPath []string
}

// NewControlPlaneVirtualIP is different from other generic handlers.
// It requires variableName and variableFieldPath to be passed from another provider specific handler.
// The code is here to be shared across different providers.
func NewControlPlaneVirtualIP(
	variableName string,
	variableFieldPath ...string,
) *ControlPlaneVirtualIP {
	return &ControlPlaneVirtualIP{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *ControlPlaneVirtualIP) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	clusterGetter mutation.ClusterGetter,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	controlPlaneEndpointVar, err := variables.Get[v1alpha1.ControlPlaneEndpointSpec](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if variables.IsNotFoundError(err) {
			log.V(5).Info("ControlPlaneEndpoint variable not defined")
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
		controlPlaneEndpointVar,
	)

	cluster, err := clusterGetter(ctx)
	if err != nil {
		log.Error(
			err,
			"failed to get cluster from ControlPlaneVirtualIP mutation handler",
		)
		return err
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			if controlPlaneEndpointVar.VirtualIPSpec == nil {
				log.V(5).Info("ControlPlane VirtualIP not set")
				// if VirtualIPSpec is not set, delete all VirtualIP providers' template files
				// as we do not want them to end up in the generated KCP
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files = deleteFiles(
					obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
					providers.VirtualIPProviderFileNames...,
				)
				return nil
			}

			var virtualIPProvider providers.Provider
			// only kube-vip is supported, but more providers can be added in the future
			if controlPlaneEndpointVar.VirtualIPSpec.Provider == v1alpha1.VirtualIPProviderKubeVIP {
				virtualIPProvider = providers.NewKubeVIPFromKCPTemplateProvider(obj)
			}

			files, preKubeadmCommands, postKubeadmCommands, generateErr := virtualIPProvider.GenerateFilesAndCommands(
				ctx,
				controlPlaneEndpointVar,
				cluster,
			)
			if generateErr != nil {
				return generateErr
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info(fmt.Sprintf(
				"adding %s static Pod file to control plane kubeadm config spec",
				virtualIPProvider.Name(),
			))

			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = mergeFiles(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				files...,
			)

			if len(preKubeadmCommands) > 0 {
				log.WithValues(
					"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
					"patchedObjectName", client.ObjectKeyFromObject(obj),
				).Info(fmt.Sprintf(
					"adding %s preKubeadmCommands to control plane kubeadm config spec",
					virtualIPProvider.Name(),
				))
				obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.PreKubeadmCommands,
					preKubeadmCommands...,
				)
			}

			if len(postKubeadmCommands) > 0 {
				log.WithValues(
					"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
					"patchedObjectName", client.ObjectKeyFromObject(obj),
				).Info(fmt.Sprintf(
					"adding %s postKubeadmCommands to control plane kubeadm config spec",
					virtualIPProvider.Name(),
				))
				obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands = append(
					obj.Spec.Template.Spec.KubeadmConfigSpec.PostKubeadmCommands,
					postKubeadmCommands...,
				)
			}

			return nil
		},
	)
}

func deleteFiles(files []bootstrapv1.File, filePathsToDelete ...string) []bootstrapv1.File {
	for i := len(files) - 1; i >= 0; i-- {
		for _, path := range filePathsToDelete {
			if files[i].Path == path {
				files = slices.Delete(files, i, i+1)
				break
			}
		}
	}

	return files
}

// mergeFiles will merge the files into the KubeadmControlPlaneTemplate,
// overriding any file with the same path and appending the rest.
func mergeFiles(files []bootstrapv1.File, filesToMerge ...bootstrapv1.File) []bootstrapv1.File {
	// replace any existing files with the same path
	for i := len(filesToMerge) - 1; i >= 0; i-- {
		for j := range files {
			if files[j].Path == filesToMerge[i].Path {
				files[j] = filesToMerge[i]
				filesToMerge = slices.Delete(filesToMerge, i, i+1)
				break
			}
		}
	}
	// append the remaining files
	files = append(files, filesToMerge...)

	return files
}
