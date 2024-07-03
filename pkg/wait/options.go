// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package wait

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Overview of how observedGeneration in used in Status and Conditions:
// https://github.com/kubernetes/community/blob/fb55d44/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

func withObservedGeneration[T client.Object](obj T, fields ...string) (bool, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return false,
			fmt.Errorf(
				"failed to convert %s %s to unstructured: %w",
				obj.GetObjectKind().GroupVersionKind(),
				client.ObjectKeyFromObject(obj),
				err,
			)
	}
	observedGeneration, found, err := unstructured.NestedInt64(
		u,
		append(fields, "observedGeneration")...)
	if err != nil {
		return false, fmt.Errorf("failed to read observedGeneration: %w", err)
	}
	if !found {
		// This means the controller has never reconciled the object.
		return false, nil
	}
	if obj.GetGeneration() > observedGeneration {
		// Spec is newer than status.
		// This usually means that the controller has not finished
		// reconciling the object since its last update.
		// Retry.
		return false, nil
	}
	// Status is at least as new as spec.
	return true, nil
}

func withStatusObservedGeneration[T client.Object](obj T) (bool, error) {
	return withObservedGeneration(obj, "status")
}

func withConditionObservedGeneration[T client.Object](obj T, name string) (bool, error) {
	return withObservedGeneration(obj, "status", "conditions", name)
}
