// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ commonhandlers.Named       = &nutanixClusterConfigVariableHandler{}
	_ mutation.DiscoverVariables = &nutanixClusterConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "NutanixClusterConfigVars"
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
	v1beta2Var := clusterv1beta2.ClusterClassVariable{
		Name:     v1alpha1.ClusterConfigVariableName,
		Required: ptr.To(true),
		Schema:   v1alpha1.NutanixClusterConfig{}.VariableSchema(),
	}
	var v1beta1Var clusterv1beta1.ClusterClassVariable
	_ = clusterv1beta1.Convert_v1beta2_ClusterClassVariable_To_v1beta1_ClusterClassVariable(
		&v1beta2Var,
		&v1beta1Var,
		nil,
	)
	resp.Variables = append(resp.Variables, v1beta1Var)
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
