// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"context"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	capdv1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	dockerclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
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
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	customImageVar, found, err := variables.Get[v1alpha1.OCIImage](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
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
				kubernetesVersion, found, err := variables.Get[string](
					vars,
					variablePath[0],
					variablePath[1:]...)
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf(
						"missing required variable: %s",
						strings.Join(variablePath, "."),
					)
				}

				customImageVar = v1alpha1.OCIImage(
					defaultKinDImageRepository + ":" + kubernetesVersion,
				)
			}

			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
				"customImage", customImageVar,
			).Info("setting customImage in control plane DockerMachineTemplate spec")

			obj.Spec.Template.Spec.CustomImage = string(customImageVar)

			return nil
		},
	)
}
