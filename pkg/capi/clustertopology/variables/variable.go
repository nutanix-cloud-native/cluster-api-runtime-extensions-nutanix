// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
)

// Get finds and parses variable to given type.
func Get[T any](
	variables map[string]apiextensionsv1.JSON,
	name string,
	fields ...string,
) (value T, found bool, err error) {
	variable, found, err := topologymutation.GetVariable(variables, name)
	if err != nil || !found {
		return value, found, err
	}

	jsonValue := variable.Raw

	if len(fields) > 0 {
		var unstr map[string]interface{}
		err = json.Unmarshal(jsonValue, &unstr)
		if err != nil {
			return value, found, err
		}

		nestedField, found, err := unstructured.NestedFieldCopy(unstr, fields...)
		if err != nil || !found {
			return value, found, err
		}

		jsonValue, err = json.Marshal(nestedField)
		if err != nil {
			return value, found, err
		}
	}

	err = json.Unmarshal(jsonValue, &value)
	return value, err == nil, err
}
