// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type unstructructuredFallback struct {
	defaultDecoder runtime.Decoder
}

var _ runtime.Decoder = unstructructuredFallback{}

func (u unstructructuredFallback) Decode(
	data []byte, defaults *schema.GroupVersionKind, into runtime.Object,
) (runtime.Object, *schema.GroupVersionKind, error) {
	if obj, gvk, err := u.defaultDecoder.Decode(data, defaults, into); err == nil {
		return obj, gvk, nil
	}

	return unstructured.UnstructuredJSONScheme.Decode(data, defaults, into)
}

func NewDecoderWithUnstructuredFallback(decoder runtime.Decoder) runtime.Decoder {
	return unstructructuredFallback{
		defaultDecoder: decoder,
	}
}
