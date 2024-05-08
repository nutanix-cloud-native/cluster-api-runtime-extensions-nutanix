// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func MustParseToUnstructured[T any](inputs ...T) []client.Object {
	return MustParseToObjects[*unstructured.Unstructured](inputs...)
}

func MustParseToObjects[T client.Object, I any](inputs ...I) []client.Object {
	var (
		objs []client.Object
		err  error
	)

	switch arr := any(inputs).(type) {
	case []string:
		objs, err = StringsToObjects[T](arr...)
	case [][]byte:
		objs, err = BytesToObjects[T](arr...)
	case []io.Reader:
		objs, err = ReadersToObjects[T](arr...)
	default:
		panic(fmt.Sprintf("unsupported type: %T", arr))
	}

	if err != nil {
		panic(fmt.Sprintf("manifest parsing failed: %v", err))
	}

	return objs
}

func StringsToUnstructured(strs ...string) ([]client.Object, error) {
	return StringsToObjects[*unstructured.Unstructured](strs...)
}

func BytesToUnstructured(bs ...[]byte) ([]client.Object, error) {
	return BytesToObjects[*unstructured.Unstructured](bs...)
}

func ReadersToUnstructured(rs ...io.Reader) ([]client.Object, error) {
	return ReadersToObjects[*unstructured.Unstructured](rs...)
}

func StringsToObjects[T client.Object](strs ...string) ([]client.Object, error) {
	rs := make([]io.Reader, 0, len(strs))
	for _, s := range strs {
		rs = append(rs, strings.NewReader(s))
	}

	return ReadersToObjects[T](rs...)
}

func BytesToObjects[T client.Object](bs ...[]byte) ([]client.Object, error) {
	rs := make([]io.Reader, 0, len(bs))
	for _, b := range bs {
		rs = append(rs, bytes.NewReader(b))
	}

	return ReadersToObjects[T](rs...)
}

func ReadersToObjects[T client.Object](rs ...io.Reader) ([]client.Object, error) {
	var objs []client.Object
	for _, r := range rs {
		decoder := yaml.NewYAMLOrJSONDecoder(r, 1024)
		for {
			o := new(T)
			err := decoder.Decode(o)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			objs = append(objs, *o)
		}
	}
	return objs, nil
}
