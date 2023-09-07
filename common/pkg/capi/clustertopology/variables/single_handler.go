// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import (
	"context"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
)

var (
	_ handlers.Named             = &singleVariableHandler{}
	_ mutation.DiscoverVariables = &singleVariableHandler{}
)

// NewAsSingleVariableHandler returns group of variables under a single name as nested
// variables.
func NewAsSingleVariableHandler(name string, vars Group, required bool) *singleVariableHandler {
	return &singleVariableHandler{
		name:      name,
		variables: vars,
		required:  required,
	}
}

type singleVariableHandler struct {
	name      string
	variables Group
	required  bool
}

func (h *singleVariableHandler) Name() string {
	return h.name
}

func (h *singleVariableHandler) DiscoverVariables(
	ctx context.Context,
	_ *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	resp.Variables = append(resp.Variables, clusterv1.ClusterClassVariable{
		Name:     h.name,
		Required: h.required,
		Schema:   h.variables.VariableSchema(),
	})
	resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
}
