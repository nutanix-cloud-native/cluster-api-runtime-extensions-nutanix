// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/apis"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capdv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	dockerclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/clusterconfig"
	dockerworkerconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/workerconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "DockerCustomImagePatch"

	defaultKinDImageRepository = "ghcr.io/mesosphere/kind-node"
)

type customImagePatchHandler struct {
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &customImagePatchHandler{}
	_ mutation.GeneratePatches = &customImagePatchHandler{}
	_ mutation.MetaMutator     = &customImagePatchHandler{}
)

func NewPatch() *customImagePatchHandler {
	return newCustomImagePatchHandler(VariableName)
}

func NewMetaPatch() *customImagePatchHandler {
	return newCustomImagePatchHandler(
		clusterconfig.MetaVariableName,
		dockerclusterconfig.DockerVariableName,
		VariableName,
	)
}

func NewMetaWorkerPatch() *customImagePatchHandler {
	return newCustomImagePatchHandler(
		workerconfig.MetaVariableName,
		dockerworkerconfig.DockerVariableName,
		VariableName,
	)
}

func newCustomImagePatchHandler(
	variableName string,
	variableFieldPath ...string,
) *customImagePatchHandler {
	return &customImagePatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *customImagePatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *customImagePatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
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
		log.V(5).Info("Docker customImage variable not defined, using default KinD node image")
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		customImageVar,
	)

	err = patches.Generate(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureWorkerMachineTemplates("v1beta1", "DockerMachineTemplate"),
		log,
		func(obj *capdv1.DockerMachineTemplate) error {
			fieldPath := []string{"builtin", "machineDeployment", "version"}

			if customImageVar == "" {
				kubernetesVersion, found, err := variables.Get[string](
					vars,
					fieldPath[0],
					fieldPath[1:]...)
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf(
						"missing required variable: %s",
						strings.Join(fieldPath, "."),
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
			).Info("setting customImage in workers DockerMachineTemplate spec")

			obj.Spec.Template.Spec.CustomImage = string(customImageVar)

			return nil
		},
	)

	if err != nil {
		return err
	}

	if h.variableName == workerconfig.MetaVariableName {
		return nil
	}

	return patches.Generate(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureControlPlaneMachines("v1beta1", "DockerMachineTemplate"),
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

func (h *customImagePatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		apis.CAPDDecoder(),
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			return h.Mutate(ctx, obj, vars, holderRef, client.ObjectKey{})
		},
	)
}
