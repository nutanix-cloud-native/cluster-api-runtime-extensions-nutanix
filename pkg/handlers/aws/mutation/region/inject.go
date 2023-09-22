// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package region

import (
	"context"
	_ "embed"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
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
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

const (
	// HandlerNamePatch is the name of the inject handler.
	HandlerNamePatch = "AWSRegionPatch"
)

type awsRegionPatchHandler struct {
	decoder           runtime.Decoder
	variableName      string
	variableFieldPath []string
}

var (
	_ commonhandlers.Named     = &awsRegionPatchHandler{}
	_ mutation.GeneratePatches = &awsRegionPatchHandler{}
	_ mutation.MetaMutater     = &awsRegionPatchHandler{}
)

func NewPatch() *awsRegionPatchHandler {
	return newImageRepositoryPatchHandler(variableName)
}

func NewMetaPatch() *awsRegionPatchHandler {
	return newImageRepositoryPatchHandler(clusterconfig.MetaVariableName, variableName)
}

func newImageRepositoryPatchHandler(
	variableName string,
	variableFieldPath ...string,
) *awsRegionPatchHandler {
	return &awsRegionPatchHandler{
		decoder: json.NewSerializerWithOptions(
			json.DefaultMetaFactory,
			nil,
			nil,
			json.SerializerOptions{},
		),
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *awsRegionPatchHandler) Name() string {
	return HandlerNamePatch
}

func (h *awsRegionPatchHandler) Mutate(
	ctx context.Context,
	obj runtime.Object,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	regionVar, found, err := variables.Get[v1alpha1.Region](
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("AWS region variable not defined")
		return nil
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		regionVar,
	)

	return patches.Generate(
		obj,
		vars,
		&holderRef,
		selectors.InfrastructureCluster("v1beta2", "AWSClusterTemplate"),
		log,
		func(obj *unstructured.Unstructured) error {
			log.WithValues(
				"patchedObjectKind", obj.GetObjectKind().GroupVersionKind().String(),
				"patchedObjectName", client.ObjectKeyFromObject(obj),
			).Info("setting region in AWSCluster spec")

			return unstructured.SetNestedField(
				obj.Object,
				string(regionVar),
				"spec",
				"template",
				"spec",
				"region",
			)
		},
	)
}

func (h *awsRegionPatchHandler) GeneratePatches(
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
