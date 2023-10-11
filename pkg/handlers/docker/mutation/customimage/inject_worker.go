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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capdv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	dockerworkerconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/workerconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

type customImageWorkerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

func NewWorkerPatch() *customImageWorkerPatchHandler {
	return newcustomImageWorkerPatchHandler(
		workerconfig.MetaVariableName,
		dockerworkerconfig.DockerVariableName,
		VariableName,
	)
}

func newcustomImageWorkerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *customImageWorkerPatchHandler {
	return &customImageWorkerPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *customImageWorkerPatchHandler) Mutate(
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
			Info("Docker customImage variable not defined for workers, using default KinD node image")
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
		selectors.InfrastructureWorkerMachineTemplates(
			capdv1.GroupVersion.Version,
			"DockerMachineTemplate",
		),
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
}
