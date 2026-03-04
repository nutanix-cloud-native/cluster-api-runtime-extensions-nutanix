// Copyright 2023 Nutanix. All rights reserved.
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
	_ commonhandlers.Named       = &awsClusterConfigVariableHandler{}
	_ mutation.DiscoverVariables = &awsClusterConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "AWSClusterConfigVars"
)

func NewVariable() *awsClusterConfigVariableHandler {
	return &awsClusterConfigVariableHandler{}
}

type awsClusterConfigVariableHandler struct{}

func (h *awsClusterConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *awsClusterConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	v1beta2Var := clusterv1beta2.ClusterClassVariable{
		Name:     v1alpha1.ClusterConfigVariableName,
		Required: ptr.To(true),
		Schema:   v1alpha1.AWSClusterConfig{}.VariableSchema(),
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
