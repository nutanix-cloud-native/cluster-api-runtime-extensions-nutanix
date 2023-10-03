// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/clusterconfig"
)

var (
	_ commonhandlers.Named       = &awsClusterConfigVariableHandler{}
	_ mutation.DiscoverVariables = &awsClusterConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "AWSClusterConfigVars"

	// AWSVariableName is the AWS config patch variable name.
	AWSVariableName = "aws"
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
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     clusterconfig.MetaVariableName,
		Required: true,
		Schema:   v1alpha1.ClusterConfigSpec{AWS: &v1alpha1.AWSSpec{}}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
