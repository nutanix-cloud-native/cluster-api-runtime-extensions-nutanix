// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"strings"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util"
)

type orderedBCC struct {
	name string

	hooks []BeforeClusterCreate
}

func (o *orderedBCC) BeforeClusterCreate(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterCreateRequest,
	resp *runtimehooksv1.BeforeClusterCreateResponse,
) {
	responses := make([]runtimehooksv1.ResponseObject, 0, len(o.hooks))
	for _, h := range o.hooks {
		hookResponse := &runtimehooksv1.BeforeClusterCreateResponse{}
		h.BeforeClusterCreate(ctx, req, hookResponse)
		if hookResponse.Status == runtimehooksv1.ResponseStatusFailure {
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = hookResponse.Message
			return
		}
		responses = append(responses, hookResponse)
	}

	aggregateSuccessfulResponses(resp, responses)
}

func (o *orderedBCC) Name() string {
	return o.name
}

func OrderedBeforeClusterCreateHook(
	name string,
	hooks ...BeforeClusterCreate,
) NamedBeforeClusterCreate {
	return &orderedBCC{
		name:  name,
		hooks: hooks,
	}
}

type orderedACPI struct {
	name  string
	hooks []AfterControlPlaneInitialized
}

func (o *orderedACPI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	responses := make([]runtimehooksv1.ResponseObject, 0, len(o.hooks))
	for _, h := range o.hooks {
		hookResponse := &runtimehooksv1.AfterControlPlaneInitializedResponse{}
		h.AfterControlPlaneInitialized(ctx, req, hookResponse)
		if hookResponse.Status == runtimehooksv1.ResponseStatusFailure {
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = hookResponse.Message
			return
		}
		responses = append(responses, hookResponse)
	}

	aggregateSuccessfulResponses(resp, responses)
}

func (o *orderedACPI) Name() string {
	return o.name
}

func OrderedAfterControlPlaneInitializedHook(
	name string, hooks ...AfterControlPlaneInitialized,
) NamedAfterControlPlaneInitialized {
	return &orderedACPI{
		name:  name,
		hooks: hooks,
	}
}

type orderedBCU struct {
	name string

	hooks []BeforeClusterUpgrade
}

func (o *orderedBCU) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	responses := make([]runtimehooksv1.ResponseObject, 0, len(o.hooks))
	for _, h := range o.hooks {
		hookResponse := &runtimehooksv1.BeforeClusterUpgradeResponse{}
		h.BeforeClusterUpgrade(ctx, req, hookResponse)
		if hookResponse.Status == runtimehooksv1.ResponseStatusFailure {
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = hookResponse.Message
			return
		}
		responses = append(responses, hookResponse)
	}

	aggregateSuccessfulResponses(resp, responses)
}

func (o *orderedBCU) Name() string {
	return o.name
}

func OrderedBeforeClusterUpgradeHook(
	name string,
	hooks ...BeforeClusterUpgrade,
) NamedBeforeClusterUpgrade {
	return &orderedBCU{
		name:  name,
		hooks: hooks,
	}
}

type orderedACPU struct {
	name string

	hooks []AfterControlPlaneUpgrade
}

func (o *orderedACPU) AfterControlPlaneUpgrade(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneUpgradeRequest,
	resp *runtimehooksv1.AfterControlPlaneUpgradeResponse,
) {
	responses := make([]runtimehooksv1.ResponseObject, 0, len(o.hooks))
	for _, h := range o.hooks {
		hookResponse := &runtimehooksv1.AfterControlPlaneUpgradeResponse{}
		h.AfterControlPlaneUpgrade(ctx, req, hookResponse)
		if hookResponse.Status == runtimehooksv1.ResponseStatusFailure {
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = hookResponse.Message
			return
		}
		responses = append(responses, hookResponse)
	}

	aggregateSuccessfulResponses(resp, responses)
}

func (o *orderedACPU) Name() string {
	return o.name
}

func OrderedAfterControlPlaneUpgradeHook(
	name string,
	hooks ...AfterControlPlaneUpgrade,
) NamedAfterControlPlaneUpgrade {
	return &orderedACPU{
		name:  name,
		hooks: hooks,
	}
}

type orderedBCD struct {
	name string

	hooks []BeforeClusterDelete
}

func (o *orderedBCD) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	responses := make([]runtimehooksv1.ResponseObject, 0, len(o.hooks))
	for _, h := range o.hooks {
		hookResponse := &runtimehooksv1.BeforeClusterDeleteResponse{}
		h.BeforeClusterDelete(ctx, req, hookResponse)
		if hookResponse.Status == runtimehooksv1.ResponseStatusFailure {
			resp.Status = runtimehooksv1.ResponseStatusFailure
			resp.Message = hookResponse.Message
			return
		}
		responses = append(responses, hookResponse)
	}

	aggregateSuccessfulResponses(resp, responses)
}

func (o *orderedBCD) Name() string {
	return o.name
}

func OrderedBeforeClusterDeleteHook(
	name string,
	hooks ...BeforeClusterDelete,
) NamedBeforeClusterDelete {
	return &orderedBCD{
		name:  name,
		hooks: hooks,
	}
}

// aggregateSuccessfulResponses aggregates all successful responses into a single response.
func aggregateSuccessfulResponses(
	aggregatedResponse runtimehooksv1.ResponseObject,
	responses []runtimehooksv1.ResponseObject,
) {
	// At this point the Status should always be ResponseStatusSuccess.
	aggregatedResponse.SetStatus(runtimehooksv1.ResponseStatusSuccess)

	// Note: As all responses have the same type we can assume now that
	// they all implement the RetryResponseObject interface.
	messages := []string{}
	for _, resp := range responses {
		aggregatedRetryResponse, ok := aggregatedResponse.(runtimehooksv1.RetryResponseObject)
		if ok {
			aggregatedRetryResponse.SetRetryAfterSeconds(util.LowestNonZeroInt32(
				aggregatedRetryResponse.GetRetryAfterSeconds(),
				resp.(runtimehooksv1.RetryResponseObject).GetRetryAfterSeconds(),
			))
		}
		if resp.GetMessage() != "" {
			messages = append(messages, resp.GetMessage())
		}
	}
	aggregatedResponse.SetMessage(strings.Join(messages, ", "))
}
