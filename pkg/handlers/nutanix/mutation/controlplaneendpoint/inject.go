// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplaneendpoint

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "controlPlaneEndpoint"
)

type nutanixControlPlaneEndpoint struct {
	variableName      string
	variableFieldPath []string
}

func NewPatch() *nutanixControlPlaneEndpoint {
	return newNutanixControlPlaneEndpoint(
		v1alpha1.ClusterConfigVariableName,
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
