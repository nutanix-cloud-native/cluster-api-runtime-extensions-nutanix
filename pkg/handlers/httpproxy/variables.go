// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxy

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/server/pkg/handlers"
)

var (
	_ handlers.NamedHandler                     = &httpProxyVariableHandler{}
	_ handlers.DiscoverVariablesMutationHandler = &httpProxyVariableHandler{}
)

const (
	// VariableName is http proxy external patch variable name.
	VariableName = "proxy"

	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "HTTPProxyVars"
)

func NewVariable() *httpProxyVariableHandler {
	return &httpProxyVariableHandler{}
}

type httpProxyVariableHandler struct{}

func (h *httpProxyVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *httpProxyVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	variable := HTTPProxyVariables{}
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     VariableName,
		Required: false,
		Schema:   variable.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// HTTPProxyVariables required for providing proxy configuration.
type HTTPProxyVariables struct {
	// HTTP proxy.
	HTTP string `json:"http"`

	// HTTPS proxy.
	HTTPS string `json:"https"`

	// No Proxy list.
	NO []string `json:"no"`
}

// VariableSchema provides Cluster Class variable schema definition.
func (HTTPProxyVariables) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"http": {
					Description: "HTTP proxy value.",
					Type:        "string",
				},
				"https": {
					Description: "HTTPS proxy value.",
					Type:        "string",
				},
				"no": {
					Description: "No Proxy list.",
					Type:        "array",
					Items: &clusterv1.JSONSchemaProps{
						Type: "string",
					},
				},
			},
		},
	}
}
