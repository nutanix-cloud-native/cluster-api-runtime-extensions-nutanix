// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"bytes"
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ToJSON(v any) []byte {
	var (
		data []byte
		err  error
	)
	if u, ok := v.(*unstructured.Unstructured); ok {
		data, err = u.MarshalJSON()
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		panic(err)
	}
	compacted := &bytes.Buffer{}
	if err := json.Compact(compacted, data); err != nil {
		panic(err)
	}
	return compacted.Bytes()
}
