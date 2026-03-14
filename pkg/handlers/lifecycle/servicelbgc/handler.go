// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
)

type ServiceLoadBalancerGC struct {
	client ctrlclient.Client
}

var (
	_ handlers.Named                = &ServiceLoadBalancerGC{}
	_ lifecycle.BeforeClusterDelete = &ServiceLoadBalancerGC{}
)

func New(client ctrlclient.Client) *ServiceLoadBalancerGC {
	return &ServiceLoadBalancerGC{client: client}
}

func (s *ServiceLoadBalancerGC) Name() string {
	return "ServiceLoadBalancerGC"
}

func (s *ServiceLoadBalancerGC) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	cluster, err := capiutils.ConvertV1Beta1ClusterToV1Beta2(&req.Cluster)
	if err != nil {
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to convert cluster: %v", err))
		return
	}
	clusterKey := ctrlclient.ObjectKeyFromObject(cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	// CAPI 1.12+ strips cluster status from the hook request. Fetch the full Cluster from the API
	// so we have phase and conditions for the cleanup decision (shouldDeleteServicesWithLoadBalancer).
	clusterWithStatus := &clusterv1beta2.Cluster{}
	if err := s.client.Get(ctx, clusterKey, clusterWithStatus); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Cluster not found (may already be deleted), allowing deletion")
			resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
			return
		}
		log.Error(err, "Failed to get cluster with status for Service LB GC decision, will retry")
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(fmt.Sprintf("failed to get cluster with status: %v", err))
		resp.SetRetryAfterSeconds(5)
		return
	}

	shouldDelete, err := shouldDeleteServicesWithLoadBalancer(clusterWithStatus)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf(
			"error determining if Services of type LoadBalancer should be deleted: %v",
			err,
		)
		return
	}

	if !shouldDelete {
		return
	}

	log.Info("Will attempt to delete Services with type LoadBalancer")
	remoteClient, err := remote.NewClusterClient(
		ctx,
		"",
		s.client,
		clusterKey,
	)
	if err != nil {
		resp.Status = runtimehooksv1.ResponseStatusFailure
		resp.Message = fmt.Sprintf(
			"error creating remote cluster client: %v",
			err,
		)
		return
	}

	err = deleteServicesWithLoadBalancer(ctx, remoteClient, log)
	switch {
	case errors.Is(err, ErrFailedToDeleteService):
		resp.SetStatus(runtimehooksv1.ResponseStatusFailure)
		resp.SetMessage(err.Error())
		resp.SetRetryAfterSeconds(5)
	case errors.Is(err, ErrServicesStillExist):
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
		resp.SetMessage(err.Error())
		resp.SetRetryAfterSeconds(5)
	default:
		resp.SetStatus(runtimehooksv1.ResponseStatusSuccess)
	}
}
