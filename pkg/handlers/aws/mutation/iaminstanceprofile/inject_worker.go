// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package iaminstanceprofile

import (
	"context"
	_ "embed"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/apis"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"

	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	awsworkerconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/workerconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

const (
	// WorkerHandlerNamePatch is the name of the inject handler.
	WorkerHandlerNamePatch = "AWSIAMInstanceProfileWorkerPatch"
)

type awsIAMInstanceProfileWorkerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &awsIAMInstanceProfileWorkerPatchHandler{}
	_ mutation.GeneratePatches = &awsIAMInstanceProfileWorkerPatchHandler{}
	_ mutation.MetaMutator     = &awsIAMInstanceProfileWorkerPatchHandler{}
)

func NewWorkerMetaPatch() *awsIAMInstanceProfileWorkerPatchHandler {
	return newAWSIAMInstanceProfileWorkerPatchHandler(
		workerconfig.MetaVariableName,
		awsworkerconfig.AWSVariableName,
		VariableName,
	)
}

func newAWSIAMInstanceProfileWorkerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *awsIAMInstanceProfileWorkerPatchHandler {
	return &awsIAMInstanceProfileWorkerPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsIAMInstanceProfileWorkerPatchHandler) Name() string {
	return WorkerHandlerNamePatch
}

func (h *awsIAMInstanceProfileWorkerPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	iamInstanceProfileVar, found, err := variables.Get[v1alpha1.IAMInstanceProfile](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("AWS IAM instance profile variable for workers not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		iamInstanceProfileVar,
	)

	return patches.Generate(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureWorkerMachineTemplates(
			"v1beta2",
			"AWSMachineTemplate",
		),
		log,
		func(obj *capav1.AWSMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting IAM instance profile in worker AWSMachineTemplate spec")

			obj.Spec.Template.Spec.IAMInstanceProfile = string(iamInstanceProfileVar)

			return nil
		},
	)
}

func (h *awsIAMInstanceProfileWorkerPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	handlers.GeneratePatches(ctx, req, resp, apis.CAPADecoder(), h.Mutate)
}
