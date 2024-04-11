// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ commonhandlers.Named       = &clusterConfigVariableHandler{}
	_ mutation.DiscoverVariables = &clusterConfigVariableHandler{}
)

const (
	// MetaVariableName is the meta cluster config patch variable name.
	MetaVariableName = "clusterConfig"

	// MetaControlPlaneConfigName is the meta control-plane config patch variable name.
	MetaControlPlaneConfigName = "controlPlane"

	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "ClusterConfigVars"
)

func NewVariable() *clusterConfigVariableHandler {
	return &clusterConfigVariableHandler{}
}

type clusterConfigVariableHandler struct{}

func (h *clusterConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *clusterConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     MetaVariableName,
		Required: false,
		Schema:   v1alpha1.GenericClusterConfig{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
