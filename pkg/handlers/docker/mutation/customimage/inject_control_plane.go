// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	capdv1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "customImage"

	defaultKinDImageRepository = "ghcr.io/mesosphere/kind-node"
)

type customImageControlPlanePatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewControlPlanePatch() *customImageControlPlanePatchHandler {
	return newCustomImageControlPlanePatchHandler(
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		dockerclusterconfig.DockerVariableName,
		VariableName,
	)
}

func newCustomImageControlPlanePatchHandler(
	variableName string,
	variableFieldPath ...string,
) *customImageControlPlanePatchHandler {
	return &customImageControlPlanePatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *customImageControlPlanePatchHandler) Mutate(
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

	customImageVar, err := variables.Get[string](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		if !variables.IsNotFoundError(err) {
			return err
		}
		log.V(5).
			Info("Docker customImage variable not defined for control-plane, using default KinD node image")
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		customImageVar,
	)

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureControlPlaneMachines(
			capdv1.GroupVersion.Version,
			"DockerMachineTemplate",
		),
		log,
		func(obj *capdv1.DockerMachineTemplate) error {
			variablePath := []string{"builtin", "controlPlane", "version"}

			if customImageVar == "" {
				kubernetesVersion, err := variables.Get[string](
					vars,
					variablePath[0],
					variablePath[1:]...)
				if err != nil && !variables.IsNotFoundError(err) {
					return err
				}

				customImageVar = defaultKinDImageRepository + ":" + kubernetesVersion
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
				"customImage", customImageVar,
			).Info("setting customImage in control plane DockerMachineTemplate spec")

			obj.Spec.Template.Spec.CustomImage = customImageVar

			return nil
		},
	)
}
