// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package serializer

import (
	"bytes"
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

	utilruntime.Must(err)

	compacted := &bytes.Buffer{}
	utilruntime.Must(json.Compact(compacted, data))
	return compacted.Bytes()
}
