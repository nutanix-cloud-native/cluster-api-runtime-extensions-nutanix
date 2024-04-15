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
	_ commonhandlers.Named       = &awsWorkerConfigVariableHandler{}
	_ mutation.DiscoverVariables = &awsWorkerConfigVariableHandler{}
)

const (
	// HandlerNameVariable is the name of the variable handler.
	HandlerNameVariable = "AWSWorkerConfigVars"
)

func NewVariable() *awsWorkerConfigVariableHandler {
	return &awsWorkerConfigVariableHandler{}
}

type awsWorkerConfigVariableHandler struct{}

func (h *awsWorkerConfigVariableHandler) Name() string {
	return HandlerNameVariable
}

func (h *awsWorkerConfigVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     workerconfig.MetaVariableName,
		Required: false,
		Schema:   v1alpha1.NewAWSWorkerConfigSpec().VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
