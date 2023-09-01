// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/openapi/patterns"
)

var (
	_ handlers.NamedHandler                     = &dnsVariableHandler{}
	_ handlers.DiscoverVariablesMutationHandler = &dnsVariableHandler{}
)

const (
	// VariableName is DNS external patch variable name.
	VariableName = "dns"

	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "DNSVars"
)

func NewVariable() *dnsVariableHandler {
	return &dnsVariableHandler{}
}

type dnsVariableHandler struct{}

func (h *dnsVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *dnsVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	variable := DNSVariable{}
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     VariableName,
		Required: false,
		Schema:   variable.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}

// DNSVariable holds DNS variables.
type DNSVariable struct {
	// ImageRepository is the container registry to pull DNS images from.
	ImageRepository string `json:"imageRepository"`
}

// VariableSchema provides Cluster Class variable schema definition.
func (DNSVariable) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "DNS defines the options for the DNS add-on installed in the cluster",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"imageRepository": {
					Description: "Container registry to pull DNS images from. This is only required if different from the image " +
						"repository used to pull all other Kubernetes component images.",
					Type:    "string",
					Pattern: patterns.Anchored(patterns.ImageRegistry),
				},
			},
		},
	}
}
