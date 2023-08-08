// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func StringsToObjects(strs ...string) ([]client.Object, error) {
	rs := make([]io.Reader, 0, len(strs))
	for _, s := range strs {
		rs = append(rs, strings.NewReader(s))
	}

	return DecodeReadersToObjects(rs...)
}

func BytesToObjects(bs ...[]byte) ([]client.Object, error) {
	rs := make([]io.Reader, 0, len(bs))
	for _, b := range bs {
		rs = append(rs, bytes.NewReader(b))
	}

	return DecodeReadersToObjects(rs...)
}

func DecodeReadersToObjects(rs ...io.Reader) ([]client.Object, error) {
	var objs []client.Object
	for _, r := range rs {
		decoder := yaml.NewYAMLOrJSONDecoder(r, 1024)
		for {
			u := &unstructured.Unstructured{}
			err := decoder.Decode(u)
			if err != nil {
				if err == io.EOF {
					break
				}
				return nil, err
			}
			objs = append(objs, u)
		}
	}
	return objs, nil
}
