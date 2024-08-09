// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
)

type BeforeClusterCreate interface {
	BeforeClusterCreate(
		context.Context,
		*runtimehooksv1.BeforeClusterCreateRequest,
		*runtimehooksv1.BeforeClusterCreateResponse,
	)
}
type NamedBeforeClusterCreate interface {
	handlers.Named
	BeforeClusterCreate
}

type AfterControlPlaneInitialized interface {
	AfterControlPlaneInitialized(
		context.Context,
		*runtimehooksv1.AfterControlPlaneInitializedRequest,
		*runtimehooksv1.AfterControlPlaneInitializedResponse,
	)
}
type NamedAfterControlPlaneInitialized interface {
	handlers.Named
	AfterControlPlaneInitialized
}

type BeforeClusterUpgrade interface {
	BeforeClusterUpgrade(
		context.Context,
		*runtimehooksv1.BeforeClusterUpgradeRequest,
		*runtimehooksv1.BeforeClusterUpgradeResponse,
	)
}
type NamedBeforeClusterUpgrade interface {
	handlers.Named
	BeforeClusterUpgrade
}

type AfterControlPlaneUpgrade interface {
	AfterControlPlaneUpgrade(
		context.Context,
		*runtimehooksv1.AfterControlPlaneUpgradeRequest,
		*runtimehooksv1.AfterControlPlaneUpgradeResponse,
	)
}
type NamedAfterControlPlaneUpgrade interface {
	handlers.Named
	AfterControlPlaneUpgrade
}

type BeforeClusterDelete interface {
	BeforeClusterDelete(
		context.Context,
		*runtimehooksv1.BeforeClusterDeleteRequest,
		*runtimehooksv1.BeforeClusterDeleteResponse,
	)
}
type NamedBeforeClusterDelete interface {
	handlers.Named
	BeforeClusterDelete
}
