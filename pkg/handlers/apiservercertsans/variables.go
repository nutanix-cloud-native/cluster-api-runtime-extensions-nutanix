// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package apiservercertsans

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/server/pkg/handlers"
)

var (
	_ handlers.NamedHandler                     = &apiServerCertSANsVariableHandler{}
	_ handlers.DiscoverVariablesMutationHandler = &apiServerCertSANsVariableHandler{}
)

const (
	// VariableName is http proxy external patch variable name.
	VariableName = "apiServerCertSANs"

	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "APIServerCertSANsVars"
)

func NewVariable() *apiServerCertSANsVariableHandler {
	return &apiServerCertSANsVariableHandler{}
}

type apiServerCertSANsVariableHandler struct{}

func (h *apiServerCertSANsVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *apiServerCertSANsVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	variable := APIServerCertSANsVariables{}
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     VariableName,
		Required: false,
		Schema:   variable.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// APIServerCertSANsVariables required for providing API server cert SANs.
type APIServerCertSANsVariables []string

// VariableSchema provides Cluster Class variable schema definition.
func (APIServerCertSANsVariables) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "array",
			Items: &clusterv1.JSONSchemaProps{
				Type: "string",
			},
		},
	}
}
