// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

const (
	// MetaVariableName is the meta cluster config patch variable name.
	MetaVariableName = "clusterConfig"

	// MetaControlPlaneConfigName is the meta control-plane config patch variable name.
	MetaControlPlaneConfigName = "controlPlane"

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
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     MetaVariableName,
		Required: true,
		Schema:   v1alpha1.GenericClusterConfig{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
