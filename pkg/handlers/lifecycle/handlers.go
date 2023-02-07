// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"

	"sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/pkg/addons/clusterresourcesets"
	k8sclient "github.com/d2iq-labs/capi-runtime-extensions/pkg/k8s/client"
)

type AddonProvider string

const (
	ClusterResourceSetAddonProvider AddonProvider = "ClusterResourceSet"
	FluxHelmReleaseAddonProvider    AddonProvider = "FluxHelmRelease"
)

// ExtensionHandlers provides a common struct shared across the lifecycle hook handlers.
type ExtensionHandlers struct {
	addonProvider AddonProvider
	client        ctrlclient.Client
}

// NewExtensionHandlers returns a ExtensionHandlers for the lifecycle hooks handlers.
func NewExtensionHandlers(
	addonProvider AddonProvider,
	client ctrlclient.Client,
) *ExtensionHandlers {
	return &ExtensionHandlers{
		addonProvider: addonProvider,
		client:        client,
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

	var err error
	switch m.addonProvider {
	case ClusterResourceSetAddonProvider:
		err = applyCNICRS(ctx, &request.Cluster, genericResourcesClient)
	case FluxHelmReleaseAddonProvider:
		// TODO Apply flux helm releases
	default:
		err = fmt.Errorf("unsupported provider: %q", m.addonProvider)
	}
	if err != nil {
		response.Status = runtimehooksv1.ResponseStatusFailure
		response.Message = err.Error()
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

func applyCNICRS(
	ctx context.Context,
	cluster *v1beta1.Cluster,
	genericResourcesClient *k8sclient.GenericResourcesClient,
) error {
	// Create CNI ClusterResourceSet and let the CAPI controller reconcile it.
	objs, err := clusterresourcesets.CNIForCluster(cluster)
	if err != nil {
		return err
	}
	return genericResourcesClient.Apply(ctx, objs)
}
