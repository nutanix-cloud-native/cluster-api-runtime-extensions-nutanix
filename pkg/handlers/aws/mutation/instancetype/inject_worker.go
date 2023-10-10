// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package instancetype

import (
	"context"
	_ "embed"

	awsworkerconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/workerconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/apis"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
)

const (
	// WorkerHandlerNamePatch is the name of the inject handler.
	WorkerHandlerNamePatch = "AWSInstanceTypeWorkerPatch"
)

type awsInstanceTypeWorkerPatchHandler struct {
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &awsInstanceTypeWorkerPatchHandler{}
	_ mutation.GeneratePatches = &awsInstanceTypeWorkerPatchHandler{}
	_ mutation.MetaMutator     = &awsInstanceTypeWorkerPatchHandler{}
)

func NewWorkerMetaPatch() *awsInstanceTypeWorkerPatchHandler {
	return newAWSInstanceTypeWorkerPatchHandler(
		workerconfig.MetaVariableName,
		awsworkerconfig.AWSVariableName,
		VariableName,
	)
}

func newAWSInstanceTypeWorkerPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *awsInstanceTypeWorkerPatchHandler {
	return &awsInstanceTypeWorkerPatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsInstanceTypeWorkerPatchHandler) Name() string {
	return WorkerHandlerNamePatch
}

func (h *awsInstanceTypeWorkerPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	instanceTypeVar, found, err := variables.Get[v1alpha1.InstanceType](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("AWS instance type variable for worker not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		instanceTypeVar,
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
			).Info("setting instance type in workers AWSMachineTemplate spec")

			obj.Spec.Template.Spec.InstanceType = string(instanceTypeVar)

			return nil
		},
	)
}

func (h *awsInstanceTypeWorkerPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	handlers.GeneratePatches(ctx, req, resp, apis.CAPADecoder(), h.Mutate)
}
