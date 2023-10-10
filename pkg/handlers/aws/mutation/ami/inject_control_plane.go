// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ami

import (
	"context"
	_ "embed"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches/selectors"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
	capav1 "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	awsclusterconfig "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/clusterconfig"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "AWSAMISpecPatch"
	VariableName     = "amiSpec"
)

type awsAMISpecPatchHandler struct {
	metaVariableName  string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named = &awsAMISpecPatchHandler{}
	_ mutation.MetaMutator = &awsAMISpecPatchHandler{}
)

func NewControlPlanePatch() *awsAMISpecPatchHandler {
	return newAWSAMISpecControlPlanePatchHandler(
		clusterconfig.MetaVariableName,
		clusterconfig.MetaControlPlaneConfigName,
		awsclusterconfig.AWSVariableName,
		VariableName,
	)
}

func newAWSAMISpecControlPlanePatchHandler(
	metaVariableName string,
	variableFieldPath ...string,
) *awsAMISpecPatchHandler {
	return &awsAMISpecPatchHandler{
		metaVariableName:  metaVariableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsAMISpecPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *awsAMISpecPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	amiSpecVar, found, err := variables.Get[v1alpha1.AMISpec](
		vars,
		h.metaVariableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("amiSpec variable not defined. Default AMI provided by CAPA will be used.")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.metaVariableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		amiSpecVar,
	)

	return patches.Generate(
		obj.(*unstructured.Unstructured),
		vars,
		&holderRef,
		selectors.InfrastructureControlPlaneMachines(capav1.GroupVersion.Version, "AWSMachineTemplate"),
		log,
		func(obj *capav1.AWSMachineTemplate) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting AMI in AWSMachineTemplate spec")

			obj.Spec.Template.Spec.AMI = capav1.AMIReference{ID: &amiSpecVar.ID}
			obj.Spec.Template.Spec.ImageLookupFormat = amiSpecVar.Format
			obj.Spec.Template.Spec.ImageLookupOrg = amiSpecVar.Org
			obj.Spec.Template.Spec.ImageLookupBaseOS = amiSpecVar.BaseOS

			return nil
		},
	)
}

func (h *awsAMISpecPatchHandler) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	topologymutation.WalkTemplates(
		ctx,
		unstructured.UnstructuredJSONScheme,
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
