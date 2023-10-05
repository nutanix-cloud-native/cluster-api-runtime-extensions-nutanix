// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workerconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ commonhandlers.Named       = &workerConfigsVariableHandler{}
	_ mutation.DiscoverVariables = &workerConfigsVariableHandler{}
)

const (
	// MetaVariableName is the meta worker config patch variable name.
	MetaVariableName = "workerConfig"

	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "WorkerConfigVars"
)

func NewVariable() *workerConfigsVariableHandler {
	return &workerConfigsVariableHandler{}
}

type workerConfigsVariableHandler struct{}

func (h *workerConfigsVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *workerConfigsVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     MetaVariableName,
		Required: false,
		Schema:   v1alpha1.GenericNodeConfig{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
