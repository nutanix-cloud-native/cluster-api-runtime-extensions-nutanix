// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	_ "embed"
	"fmt"
	"github.com/go-logr/logr"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capdv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	dockerclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	// ControlPlaneHandlerNamePatch is the name of the inject handler.
	ControlPlaneHandlerNamePatch = "DockerCustomImageControlPlanePatch"

	defaultKinDImageRepository = "ghcr.io/mesosphere/kind-node"
)

type customImageControlPlanePatchHandler struct {
	*handlers.GenericPatchHandler[*capdv1.DockerMachineTemplate]
}

var (
	_ commonhandlers.Named     = &customImageControlPlanePatchHandler{}
	_ mutation.GeneratePatches = &customImageControlPlanePatchHandler{}
	_ mutation.MetaMutator     = &customImageControlPlanePatchHandler{}
)

func NewControlPlanePatch() *customImageControlPlanePatchHandler {
	return newCustomImageControlPlanePatchHandler(VariableName)
}

func NewControlPlaneMetaPatch() *customImageControlPlanePatchHandler {
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
		handlers.NewGenericPatchHandler[*capdv1.DockerMachineTemplate](
			WorkerHandlerNamePatch,
			variableFunc,
			selectors.InfrastructureControlPlaneMachines("v1beta1", "DockerMachineTemplate"),
			controlPlaneMutateFunc,
			variableName,
			variableFieldPath...,
		).AlwaysMutate(),
	}
}

func variableFunc(vars map[string]apiextensionsv1.JSON, name string, fields ...string) (any, bool, error) {
	return variables.Get[v1alpha1.OCIImage](vars, name, fields...)
}

func controlPlaneMutateFunc(
	log logr.Logger,
	vars map[string]apiextensionsv1.JSON,
	patchVar any,
) func(obj *capdv1.DockerMachineTemplate) error {
	return func(obj *capdv1.DockerMachineTemplate) error {
		customImageVar := patchVar.(v1alpha1.OCIImage)

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
	}
}
