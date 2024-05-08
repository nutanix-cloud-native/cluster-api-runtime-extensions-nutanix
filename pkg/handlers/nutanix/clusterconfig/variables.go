// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
)

var (
	_ commonhandlers.Named       = &nutanixClusterConfigVariableHandler{}
	_ mutation.DiscoverVariables = &nutanixClusterConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "NutanixClusterConfigVars"

	// NutanixVariableName is the Nutanix config patch variable name.
	NutanixVariableName = "nutanix"
)

func NewVariable() *nutanixClusterConfigVariableHandler {
	return &nutanixClusterConfigVariableHandler{}
}

type nutanixClusterConfigVariableHandler struct{}

func (h *nutanixClusterConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *nutanixClusterConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     clusterconfig.MetaVariableName,
		Required: true,
		Schema:   v1alpha1.NutanixClusterConfig{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
