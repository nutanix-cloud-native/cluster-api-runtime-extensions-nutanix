// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patches

import (
	"bytes"
	"encoding/json"
	"fmt"

	jsonpatchapply "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/matchers"
)

// MutateIfApplicable applies the typed mutFn function to the unstructured input if the input
// matches the holderRef and patchSelector. This is used by handlers to reduce the boilerplate
// required there.
//
// This function makes it easier for handlers to work with typed objects, while ensuring that
// patches properly work with minimal input documents. While it would feel more natural to work
// purely with typed objects, due to the semantics of `omitempty` and non-pointer struct fields,
// it is required to work with unstructured objects for input and output. This leads to minimal
// patches, regardless of the completeness of the input type.
func MutateIfApplicable[T runtime.Object](
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef *runtimehooksv1.HolderReference,
	patchSelector clusterv1.PatchSelector,
	log logr.Logger,
	mutFn func(T) error,
) error {
	// Check the object matches the selector.
	if !matchers.MatchesSelector(patchSelector, obj, holderRef, vars) {
		log.V(5).WithValues("selector", patchSelector).Info("not matching selector")
		return nil
	}

	// Convert the unstructured object to the expected type.
	var typed T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(
		obj.Object,
		&typed,
		true,
	); err != nil {
		log.V(5).WithValues(
			"objKind", obj.GetObjectKind().GroupVersionKind(),
			"expectedType", fmt.Sprintf("%T", &typed),
		).Info("not matching type")
		return nil
	}

	// Create a deep clone of the typed object.
	unmodifiedTyped := typed.DeepCopyObject()

	// Apply the mutation.
	if err := mutFn(typed); err != nil {
		log.WithValues(
			"objKind", obj.GetObjectKind().GroupVersionKind(),
			"objName", obj.GetName(),
		).Error(err, "failed to apply mutation")
		return fmt.Errorf("failed to apply mutation: %w", err)
	}

	// Create JSON patches of the modifications.
	serializer := kjson.NewSerializerWithOptions(
		kjson.DefaultMetaFactory,
		nil,
		nil,
		kjson.SerializerOptions{},
	)
	var unmodifiedTypedBuf, modifiedTypedBuf bytes.Buffer
	if err := serializer.Encode(unmodifiedTyped, &unmodifiedTypedBuf); err != nil {
		return fmt.Errorf("failed to serialize unmodified typed object: %w", err)
	}
	if err := serializer.Encode(typed, &modifiedTypedBuf); err != nil {
		return fmt.Errorf("failed to serialize modified typed object: %w", err)
	}

	jsonOps, err := jsonpatch.CreatePatch(unmodifiedTypedBuf.Bytes(), modifiedTypedBuf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to create JSON patches for modified typed object: %w", err)
	}

	// Apply the patches to the original unstructured input.
	jsonOpsBytes, err := json.Marshal(jsonOps)
	if err != nil {
		return fmt.Errorf("failed to marshal json patch: %w", err)
	}

	jsonPatch, err := jsonpatchapply.DecodePatch(jsonOpsBytes)
	if err != nil {
		return fmt.Errorf("failed to decode json patch (RFC6902): %w", err)
	}

	marshalledInputObj, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal unstructured input object: %w", err)
	}

	patchedTemplate, err := jsonPatch.ApplyWithOptions(
		marshalledInputObj,
		&jsonpatchapply.ApplyOptions{
			EnsurePathExistsOnAdd:    true,
			AllowMissingPathOnRemove: true,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to apply JSON patches to input: %w", err)
	}

	// Finally unmarshal patched input into the original object. Mutation complete!
	if err := obj.UnmarshalJSON(patchedTemplate); err != nil {
		return fmt.Errorf("failed to unmarshal patched unstructured input: %w", err)
	}

	return nil
}
