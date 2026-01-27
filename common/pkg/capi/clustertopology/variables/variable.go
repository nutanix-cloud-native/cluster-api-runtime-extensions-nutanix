// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
)

// fieldNotFoundError is used when a variable is not found.
type fieldNotFoundError struct {
	fieldPath    []string
	variableName string
}

func (e fieldNotFoundError) Error() string {
	return fmt.Sprintf(
		"field %q not found in variable %q",
		strings.Join(e.fieldPath, "."),
		e.variableName,
	)
}

func IsNotFoundError(err error) bool {
	return topologymutation.IsNotFoundError(err) || errors.As(err, &fieldNotFoundError{})
}

func IsFieldNotFoundError(err error) bool {
	return errors.As(err, &fieldNotFoundError{})
}

// Get finds and parses variable to given type.
func Get[T any](
	variables map[string]apiextensionsv1.JSON,
	name string,
	fields ...string,
) (value T, err error) {
	variable, err := topologymutation.GetVariable(variables, name)
	if err != nil {
		return value, err
	}

	jsonValue := variable.Raw

	if len(fields) > 0 {
		var unstr map[string]interface{}
		err = json.Unmarshal(jsonValue, &unstr)
		if err != nil {
			return value, err
		}

		nestedField, found, err := unstructured.NestedFieldCopy(unstr, fields...)
		if err != nil {
			return value, err
		}
		if !found {
			return value, fieldNotFoundError{fieldPath: fields, variableName: name}
		}

		jsonValue, err = json.Marshal(nestedField)
		if err != nil {
			return value, err
		}
	}

	err = json.Unmarshal(jsonValue, &value)
	return value, err
}

// ClusterVariablesToVariablesMap converts a list of ClusterVariables to a map of JSON (name is the map key).
// See: https://github.com/kubernetes-sigs/cluster-api/blob/v1.5.1/internal/controllers/topology/cluster/patches/variables/variables.go#L445
//
//nolint:lll // Long URLs in comments above. Adding nolint:lll here because it doesn't work in comment lines. See: https://github.com/golangci/golangci-lint/issues/3983
func ClusterVariablesToVariablesMap(
	variables []clusterv1beta1.ClusterVariable,
) map[string]apiextensionsv1.JSON {
	if variables == nil {
		return nil
	}

	variablesMap := make(map[string]apiextensionsv1.JSON, len(variables))
	for i := range variables {
		variablesMap[variables[i].Name] = variables[i].Value
	}
	return variablesMap
}
