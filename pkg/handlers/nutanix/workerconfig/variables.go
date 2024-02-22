// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workerconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/api/v1alpha1"
	commonhandlers "github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/workerconfig"
)

var (
	_ commonhandlers.Named       = &nutanixWorkerConfigVariableHandler{}
	_ mutation.DiscoverVariables = &nutanixWorkerConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "NutanixWorkerConfigVars"

	// NutanixVariableName is the Nutanix config patch variable name.
	NutanixVariableName = "nutanix"
)

func NewVariable() *nutanixWorkerConfigVariableHandler {
	return &nutanixWorkerConfigVariableHandler{}
}

type nutanixWorkerConfigVariableHandler struct{}

func (h *nutanixWorkerConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *nutanixWorkerConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     workerconfig.MetaVariableName,
		Required: false,
		Schema:   v1alpha1.NodeConfigSpec{Nutanix: &v1alpha1.NutanixNodeSpec{}}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
