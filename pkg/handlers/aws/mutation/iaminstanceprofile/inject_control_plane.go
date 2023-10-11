// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package iaminstanceprofile

import (
	"context"
	_ "embed"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	// VariableName is the external patch variable name.
	VariableName = "iamInstanceProfile"

	// ControlPlaneHandlerNamePatch is the name of the inject handler.
	ControlPlaneHandlerNamePatch = "AWSIAMInstanceProfileControlPlanePatch"
)

type awsIAMInstanceProfileControlPlanePatchHandler struct {
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &awsIAMInstanceProfileControlPlanePatchHandler{}
	_ mutation.GeneratePatches = &awsIAMInstanceProfileControlPlanePatchHandler{}
	_ mutation.MetaMutator     = &awsIAMInstanceProfileControlPlanePatchHandler{}
)

func NewControlPlaneMetaPatch() *awsIAMInstanceProfileControlPlanePatchHandler {
	return newAWSIAMInstanceProfileControlPlanePatchHandler(
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		awsclusterconfig.AWSVariableName,
		VariableName,
	)
}

func newAWSIAMInstanceProfileControlPlanePatchHandler(
	variableName string,
	variableFieldPath ...string,
) *awsIAMInstanceProfileControlPlanePatchHandler {
	return &awsIAMInstanceProfileControlPlanePatchHandler{
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsIAMInstanceProfileControlPlanePatchHandler) Name() string {
	return ControlPlaneHandlerNamePatch
}

func (h *awsIAMInstanceProfileControlPlanePatchHandler) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
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
		log.V(5).Info("AWS IAM instance profile variable for control-plane not defined")
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

	return patches.MutateIfApplicable(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureControlPlaneMachines(
			"v1beta2",
			"AWSMachineTemplate",
		),
		log,
		func(obj *capav1.AWSMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting IAM instance profile in control plane AWSMachineTemplate spec")

			obj.Spec.Template.Spec.IAMInstanceProfile = string(iamInstanceProfileVar)

			return nil
		},
	)
}

func (h *awsIAMInstanceProfileControlPlanePatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	handlers.GeneratePatches(ctx, req, resp, h.Mutate)
}
