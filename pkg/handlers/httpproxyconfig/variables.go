// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxyconfig

import (
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
)

// GetVariable finds and parses variable to given type.
func GetVariable[T any](variables map[string]apiextensionsv1.JSON, name string) (T, bool, error) {
	var v T
	variable, found, err := topologymutation.GetVariable(variables, name)
	if err != nil || !found {
		return v, found, err
	}

	err = json.Unmarshal(variable.Raw, &v)
	return v, err == nil, err
}

// HTTPProxyVariables required for providing proxy configuration for CAPI controllers.
type HTTPProxyVariables struct {
	// HTTP proxy for CAPI controllers.
	HTTP string `json:"http"`

	// HTTPS proxy for CAPI controllers.
	HTTPS string `json:"https"`

	// No Proxy list for CAPI controllers.
	NO []string `json:"no"`
}

// VariableSchema provides Cluster Class variable schema definition.
func (HTTPProxyVariables) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"http": {
					Description: "HTTP proxy for CAPI controllers.",
					Type:        "string",
				},
				"https": {
					Description: "HTTPS proxy for CAPI controllers.",
					Type:        "string",
				},
				"no": {
					Description: "No Proxy list for CAPI controllers.",
					Type:        "array",
					Items: &clusterv1.JSONSchemaProps{
						Type: "string",
					},
				},
			},
		},
	}
}
