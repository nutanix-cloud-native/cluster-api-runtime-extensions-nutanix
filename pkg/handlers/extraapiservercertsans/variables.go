// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/openapi/patterns"
)

var (
	_ handlers.NamedHandler                     = &extraAPIServerCertSANsVariableHandler{}
	_ handlers.DiscoverVariablesMutationHandler = &extraAPIServerCertSANsVariableHandler{}
)

const (
	// VariableName is http proxy external patch variable name.
	VariableName = "extraAPIServerCertSANs"

	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "ExtraAPIServerCertSANsVars"
)

func NewVariable() *extraAPIServerCertSANsVariableHandler {
	return &extraAPIServerCertSANsVariableHandler{}
}

type extraAPIServerCertSANsVariableHandler struct{}

func (h *extraAPIServerCertSANsVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *extraAPIServerCertSANsVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	variable := ExtraAPIServerCertSANsVariables{}
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     VariableName,
		Required: false,
		Schema:   variable.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// ExtraAPIServerCertSANsVariables required for providing API server cert SANs.
type ExtraAPIServerCertSANsVariables []string

// VariableSchema provides Cluster Class variable schema definition.
func (ExtraAPIServerCertSANsVariables) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Extra Subject Alternative Names for the API Server signing cert",
			Type:        "array",
			UniqueItems: true,
			Items: &clusterv1.JSONSchemaProps{
				Type:    "string",
				Pattern: patterns.Anchored(patterns.DNS1123Subdomain),
			},
		},
	}
}
