// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package servicelbgc

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/lifecycle"
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
	clusterKey := ctrlclient.ObjectKeyFromObject(&req.Cluster)

	log := ctrl.LoggerFrom(ctx).WithValues(
		"cluster",
		clusterKey,
	)

	shouldDelete, err := shouldDeleteServicesWithLoadBalancer(&req.Cluster)
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
