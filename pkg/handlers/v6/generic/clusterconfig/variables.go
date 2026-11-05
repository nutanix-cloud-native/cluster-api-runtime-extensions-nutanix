// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/variables"
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "GenericClusterConfigVars"
)

func NewVariable() *genericClusterConfigVariableHandler {
	return &genericClusterConfigVariableHandler{}
}

type genericClusterConfigVariableHandler struct{}

func (h *genericClusterConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *genericClusterConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	v1beta2Var := clusterv1.ClusterClassVariable{
		Name:     v1alpha1.ClusterConfigVariableName,
		Required: new(true),
		Schema:   variables.GenericClusterConfig,
	}
	resp.Variables = append(resp.Variables, v1beta2Var)
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
