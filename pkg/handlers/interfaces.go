// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	"github.com/spf13/pflag"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

type NamedHandler interface {
	Name() string
}

type FlagConfigurableHandler interface {
	AddFlags(prefix string, fs *pflag.FlagSet)
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
		*runtimehooksv1.AfterControlPlaneInitializedRequest,
		*runtimehooksv1.AfterControlPlaneInitializedResponse,
	)
}
type BeforeClusterUpgradeLifecycleHandler interface {
	BeforeClusterUpgrade(
		context.Context,
		*runtimehooksv1.BeforeClusterUpgradeRequest,
		*runtimehooksv1.BeforeClusterUpgradeResponse,
	)
}
type AfterControlPlaneUpgradeLifecycleHandler interface {
	AfterControlPlaneUpgrade(
		context.Context,
		*runtimehooksv1.AfterControlPlaneUpgradeRequest,
		*runtimehooksv1.AfterControlPlaneUpgradeResponse,
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
