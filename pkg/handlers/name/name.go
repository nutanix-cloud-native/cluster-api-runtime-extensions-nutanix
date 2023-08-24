// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package name

import (
	"context"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers"
)

type DiscoverVariablesNamed interface {
	handlers.DiscoverVariablesMutationHandler
	handlers.NamedHandler
}

func NewDiscoveryVariables(ng NameGenerator, h DiscoverVariablesNamed) DiscoverVariablesNamed {
	return &discoverVariables{
		handler:       h,
		nameGenerator: ng,
	}
}

type discoverVariables struct {
	handler       DiscoverVariablesNamed
	nameGenerator NameGenerator
}

func (n discoverVariables) Name() string {
	return n.nameGenerator(n.handler.Name())
}

func (n discoverVariables) DiscoverVariables(
	ctx context.Context,
	req *runtimehooksv1.DiscoverVariablesRequest,
	resp *runtimehooksv1.DiscoverVariablesResponse,
) {
	n.handler.DiscoverVariables(ctx, req, resp)
}

type GeneratePatchesNamed interface {
	handlers.GeneratePatchesMutationHandler
	handlers.NamedHandler
}

func NewGeneratePatches(ng NameGenerator, h GeneratePatchesNamed) GeneratePatchesNamed {
	return &generatePatches{
		handler:       h,
		nameGenerator: ng,
	}
}

type generatePatches struct {
	handler       GeneratePatchesNamed
	nameGenerator NameGenerator
}

func (n generatePatches) Name() string {
	return n.nameGenerator(n.handler.Name())
}

func (n generatePatches) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	n.handler.GeneratePatches(ctx, req, resp)
}
