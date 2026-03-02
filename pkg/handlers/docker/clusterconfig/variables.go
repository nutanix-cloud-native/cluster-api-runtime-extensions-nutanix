// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ commonhandlers.Named       = &dockerClusterConfigVariableHandler{}
	_ mutation.DiscoverVariables = &dockerClusterConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "DockerClusterConfigVars"
)

func NewVariable() *dockerClusterConfigVariableHandler {
	return &dockerClusterConfigVariableHandler{}
}

type dockerClusterConfigVariableHandler struct{}

func (h *dockerClusterConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *dockerClusterConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     v1alpha1.ClusterConfigVariableName,
		Required: true,
		Schema:   v1alpha1.DockerClusterConfig{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
