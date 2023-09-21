// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package extraapiservercertsans

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ handlers.Named             = &extraAPIServerCertSANsVariableHandler{}
	_ mutation.DiscoverVariables = &extraAPIServerCertSANsVariableHandler{}
)

const (
	// variableName is http proxy external patch variable name.
	variableName = "extraAPIServerCertSANs"

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
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     variableName,
		Required: false,
		Schema:   v1alpha1.ExtraAPIServerCertSANs{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
