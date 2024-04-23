// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common/controlplaneendpoint/virtualip"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "controlPlaneEndpoint"
)

type nutanixControlPlaneEndpoint struct {
	virtualIPProvider virtualip.Provider

	variableName      string
	variableFieldPath []string
}

func NewPatch() *nutanixControlPlaneEndpoint {
	return newNutanixControlPlaneEndpoint(
		clusterconfig.MetaVariableName,
		v1alpha1.NutanixVariableName,
		VariableName,
	)
}

func newNutanixControlPlaneEndpoint(
	variableName string,
	variableFieldPath ...string,
) *nutanixControlPlaneEndpoint {
	return &nutanixControlPlaneEndpoint{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *nutanixControlPlaneEndpoint) WithVirtualIPProvider(
	virtualIPProvider virtualip.Provider,
) *nutanixControlPlaneEndpoint {
	h.virtualIPProvider = virtualIPProvider
	return h
}

func (h *nutanixControlPlaneEndpoint) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
	_ mutation.ClusterGetter,
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
			log.V(5).Info("Nutanix ControlPlaneEndpoint variable not defined")
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

	if err := patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.ControlPlane(),
		log,
		func(obj *controlplanev1.KubeadmControlPlaneTemplate) error {
			if h.virtualIPProvider == nil {
				return nil
			}

			virtualIPProviderFile, virtualIPProviderErr := h.virtualIPProvider.GetFile(ctx, controlPlaneEndpointVar)
			if virtualIPProviderErr != nil {
				return virtualIPProviderErr
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("adding kube-vip static Pod file to control plane kubeadm config spec")
			obj.Spec.Template.Spec.KubeadmConfigSpec.Files = append(
				obj.Spec.Template.Spec.KubeadmConfigSpec.Files,
				*virtualIPProviderFile,
			)
			return nil
		},
	); err != nil {
		return err
	}

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster(capxv1.GroupVersion.Version, "NutanixClusterTemplate"),
		log,
		func(obj *capxv1.NutanixClusterTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting controlPlaneEndpoint in NutanixCluster spec")

			obj.Spec.Template.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
				Host: controlPlaneEndpointVar.Host,
				Port: controlPlaneEndpointVar.Port,
			}

			return nil
		},
	)
}
