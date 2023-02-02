// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/addons"
	k8sclient "github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/client"
)

// ExtensionHandlers provides a common struct shared across the lifecycle hook handlers.
type ExtensionHandlers struct {
	client ctrlclient.Client
}

// NewExtensionHandlers returns a ExtensionHandlers for the lifecycle hooks handlers.
func NewExtensionHandlers(client ctrlclient.Client) *ExtensionHandlers {
	return &ExtensionHandlers{
		client: client,
	}
}

func (m *ExtensionHandlers) DoBeforeClusterCreate(
	ctx context.Context,
	request *runtimehooksv1.BeforeClusterCreateRequest,
	response *runtimehooksv1.BeforeClusterCreateResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterCreate is called")
}

func (m *ExtensionHandlers) DoAfterControlPlaneInitialized(
	ctx context.Context,
	request *runtimehooksv1.AfterControlPlaneInitializedRequest,
	response *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("AfterControlPlaneInitialized is called")

	genericResourcesClient := k8sclient.NewGenericResourcesClient(m.client, log)

	// Create CNI ClusterResourceSet and let the CAPI controller reconcile it
	objs, err := addons.CNIForCluster(&request.Cluster)
	if err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}
	err = genericResourcesClient.Create(ctx, objs)
	if err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
		return
	}
}

func (m *ExtensionHandlers) DoBeforeClusterUpgrade(
	ctx context.Context,
	request *runtimehooksv1.BeforeClusterUpgradeRequest,
	response *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterUpgrade is called")
}

func (m *ExtensionHandlers) DoBeforeClusterDelete(
	ctx context.Context,
	request *runtimehooksv1.BeforeClusterDeleteRequest,
	response *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("BeforeClusterDelete is called")
}
