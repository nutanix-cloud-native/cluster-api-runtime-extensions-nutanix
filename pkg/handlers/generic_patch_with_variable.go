// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/patches"
)

type GenericPatchHandler[T runtime.Object] struct {
	handlerName string

	variableFn func(map[string]apiextensionsv1.JSON, string, ...string) (any, bool, error)

	patchSelector clusterv1.PatchSelector
	alwaysMutate  bool
	mutateFn      func(logr.Logger, map[string]apiextensionsv1.JSON, any) func(T) error

	variableName      string
	variableFieldPath []string
}

func NewGenericPatchHandler[T runtime.Object](
	handlerName string,
	variableFn func(map[string]apiextensionsv1.JSON, string, ...string) (any, bool, error),
	patchSelector clusterv1.PatchSelector,
	mutateFn func(logr.Logger, map[string]apiextensionsv1.JSON, any) func(T) error,
	variableName string,
	variableFieldPath ...string,
) *GenericPatchHandler[T] {
	return &GenericPatchHandler[T]{
		handlerName:       handlerName,
		variableFn:        variableFn,
		patchSelector:     patchSelector,
		mutateFn:          mutateFn,
		variableName:      variableName,
		variableFieldPath: variableFieldPath,
	}
}

func (h *GenericPatchHandler[T]) AlwaysMutate() *GenericPatchHandler[T] {
	h.alwaysMutate = true
	return h
}

func (h *GenericPatchHandler[_]) Name() string {
	return h.handlerName
}

func (h *GenericPatchHandler[_]) Mutate(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	_ client.ObjectKey,
) error {
	log := ctrl.LoggerFrom(ctx).WithValues(
		"holderRef", holderRef,
	)

	variable, found, err := h.variableFn(
		vars,
		h.variableName,
		h.variableFieldPath...,
	)
	if err != nil {
		return err
	}
	if !found {
		log.V(5).Info("Variable not defined", "handler", h.handlerName, "variableName", h.variableName)
		if !h.alwaysMutate {
			return nil
		}
	}

	log = log.WithValues(
		"variableName",
		h.variableName,
		"variableFieldPath",
		h.variableFieldPath,
		"variableValue",
		variable,
	)

	return patches.MutateIfApplicable(
		obj, vars, &holderRef, h.patchSelector, log, h.mutateFn(log, vars, variable),
	)
}

func (h *GenericPatchHandler[_]) GeneratePatches(
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
			return h.Mutate(
				ctx,
				obj.(*unstructured.Unstructured),
				vars,
				holderRef,
				client.ObjectKey{},
			)
		},
	)
}
