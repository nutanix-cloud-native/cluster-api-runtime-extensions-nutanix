// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package patches

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	jsonpatchapply "github.com/evanphx/json-patch/v5"
	"github.com/go-logr/logr"
	"gomodules.xyz/jsonpatch/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
		return fmt.Errorf(
			"failed to convert unstructured object (%s) to typed object: %w",
			obj.GetObjectKind().GroupVersionKind().String(),
			err,
		)
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

	// jsonpatch.CreatePatch can emit "replace" operations for paths that don't exist in the (possibly minimal)
	// input object. Applying such a patch to the minimal input fails with:
	//   replace operation does not apply: doc is missing key: /some/path
	//
	// To preserve the "works with minimal templates" behavior, convert "replace" to "add" when the input
	// object does not currently have the path.
	for i := range jsonOps {
		if strings.EqualFold(jsonOps[i].Operation, "replace") && !jsonPointerExists(obj.Object, jsonOps[i].Path) {
			jsonOps[i].Operation = "add"
		}
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

// jsonPointerExists returns true if the given RFC6901 JSON pointer exists in the document.
// It supports map keys and array indices.
func jsonPointerExists(doc any, pointer string) bool {
	if pointer == "" || pointer == "/" {
		return true
	}
	if !strings.HasPrefix(pointer, "/") {
		return false
	}

	cur := doc
	parts := strings.Split(pointer, "/")[1:]
	for _, raw := range parts {
		tok := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")

		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[tok]
			if !ok {
				return false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(tok)
			if err != nil || idx < 0 || idx >= len(node) {
				return false
			}
			cur = node[idx]
		default:
			return false
		}
	}
	return true
}
