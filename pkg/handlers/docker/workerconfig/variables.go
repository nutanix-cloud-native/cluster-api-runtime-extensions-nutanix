// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workerconfig

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	commonhandlers "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/workerconfig"
)

var (
	_ commonhandlers.Named       = &dockerWorkerConfigVariableHandler{}
	_ mutation.DiscoverVariables = &dockerWorkerConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "DockerWorkerConfigVars"

	// DockerVariableName is the Docker config patch variable name.
	DockerVariableName = "docker"
)

func NewVariable() *dockerWorkerConfigVariableHandler {
	return &dockerWorkerConfigVariableHandler{}
}

type dockerWorkerConfigVariableHandler struct{}

func (h *dockerWorkerConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *dockerWorkerConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     workerconfig.MetaVariableName,
		Required: false,
		Schema:   v1alpha1.NodeConfigSpec{Docker: &v1alpha1.DockerNodeSpec{}}.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
