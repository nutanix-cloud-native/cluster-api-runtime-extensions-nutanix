// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

type DiscoverVariables interface {
	DiscoverVariables(
		context.Context,
		*runtimehooksv1.DiscoverVariablesRequest,
		*runtimehooksv1.DiscoverVariablesResponse,
	)
}
type GeneratePatches interface {
	GeneratePatches(
		context.Context,
		*runtimehooksv1.GeneratePatchesRequest,
		*runtimehooksv1.GeneratePatchesResponse,
	)
}
type ValidateTopology interface {
	ValidateTopology(
		context.Context,
		*runtimehooksv1.ValidateTopologyRequest,
		*runtimehooksv1.ValidateTopologyResponse,
	)
}
