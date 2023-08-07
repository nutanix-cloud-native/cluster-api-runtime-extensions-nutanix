// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

type NamedHandler interface {
	Name() string
}

type BeforeClusterCreateLifecycleHandler interface {
	BeforeClusterCreate(
		context.Context,
		*runtimehooksv1.BeforeClusterCreateRequest,
		*runtimehooksv1.BeforeClusterCreateResponse,
	)
}
type AfterControlPlaneInitializedLifecycleHandler interface {
	AfterControlPlaneInitialized(
		context.Context,
		*runtimehooksv1.BeforeClusterCreateRequest, *runtimehooksv1.BeforeClusterCreateResponse)
}
type BeforeClusterUpgradeLifecycleHandler interface {
	BeforeClusterUpgrade(
		context.Context,
		*runtimehooksv1.BeforeClusterUpgradeRequest,
		*runtimehooksv1.BeforeClusterUpgradeResponse,
	)
}
type BeforeClusterDeleteLifecycleHandler interface {
	BeforeClusterDelete(
		context.Context,
		*runtimehooksv1.BeforeClusterDeleteRequest,
		*runtimehooksv1.BeforeClusterDeleteResponse,
	)
}

type DiscoverVariablesMutationHandler interface {
	DiscoverVariables(
		context.Context,
		*runtimehooksv1.DiscoverVariablesRequest,
		*runtimehooksv1.DiscoverVariablesResponse,
	)
}
type GeneratePatchesMutationHandler interface {
	GeneratePatches(
		context.Context,
		*runtimehooksv1.GeneratePatchesRequest,
		*runtimehooksv1.GeneratePatchesResponse,
	)
}
type ValidateTopologyMutationHandler interface {
	ValidateTopology(
		context.Context,
		*runtimehooksv1.ValidateTopologyRequest,
		*runtimehooksv1.ValidateTopologyResponse,
	)
}
