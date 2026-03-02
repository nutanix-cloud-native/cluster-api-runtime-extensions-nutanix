// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workerconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ commonhandlers.Named       = &eksWorkerConfigVariableHandler{}
	_ mutation.DiscoverVariables = &eksWorkerConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "EKSWorkerConfigVars"
)

func NewVariable() *eksWorkerConfigVariableHandler {
	return &eksWorkerConfigVariableHandler{}
}

type eksWorkerConfigVariableHandler struct{}

func (h *eksWorkerConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *eksWorkerConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     v1alpha1.WorkerConfigVariableName,
		Required: false,
		Schema:   v1alpha1.EKSWorkerNodeConfig{}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
