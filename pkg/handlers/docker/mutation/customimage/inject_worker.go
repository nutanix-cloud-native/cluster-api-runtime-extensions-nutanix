// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package customimage

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capdv1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta1"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	dockerworkerconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/docker/workerconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

const (
	// WorkerHandlerNamePatch is the name of the inject handler.
	WorkerHandlerNamePatch = "DockerCustomImageWorkerPatch"
)

type customImageWorkerPatchHandler struct {
	*handlers.GenericPatchHandler[*capdv1.DockerMachineTemplate]
}

var (
	_ commonhandlers.Named     = &customImageWorkerPatchHandler{}
	_ mutation.GeneratePatches = &customImageWorkerPatchHandler{}
	_ mutation.MetaMutator     = &customImageWorkerPatchHandler{}
)

func NewWorkerPatch() *customImageWorkerPatchHandler {
	return newcustomImageWorkerPatchHandler(VariableName)
}

func NewWorkerMetaPatch() *customImageWorkerPatchHandler {
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
		handlers.NewGenericPatchHandler[*capdv1.DockerMachineTemplate](
			WorkerHandlerNamePatch,
			variableFunc,
			selectors.InfrastructureWorkerMachineTemplates("v1beta1", "DockerMachineTemplate"),
			workerMutateFunc,
			variableName,
			variableFieldPath...,
		).AlwaysMutate(),
	}
}

func workerMutateFunc(
	log logr.Logger,
	vars map[string]apiextensionsv1.JSON,
	patchVar any,
) func(obj *capdv1.DockerMachineTemplate) error {
	return func(obj *capdv1.DockerMachineTemplate) error {
		customImageVar := patchVar.(v1alpha1.OCIImage)

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
	}
}
