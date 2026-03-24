// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"reflect"
	"strings"
	"sync"

	"github.com/samber/lo"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/util"
)

// runHooksInParallel runs all the given hook functions in parallel and aggregates the responses.
// Every hook is run regardless of whether other hooks fail or not, with results aggregated only
// after every hook has returned.
func runHooksInParallel[T runtimehooksv1.RequestObject, U runtimehooksv1.ResponseObject](
	ctx context.Context, hookFuncs []func(context.Context, T, U), request T, response U,
) {
	responseChan := make(chan U, len(hookFuncs))

	responsePtrType := reflect.TypeFor[U]().Elem()

	// Create a wait group to wait used to wait for all hooks to finish.
	var wg sync.WaitGroup
	// Add the number of hooks to the wait group.
	wg.Add(len(hookFuncs))

	// Run each hook in a new goroutine, send the response to the response channel and decrement the
	// wait group counter when the hook finishes.
	for _, f := range hookFuncs {
		// Create a new instance of the response object type for each hook. Due to the use of generics,
		// we need to use reflection to create a new instance of the response object type.
		hookResponse := reflect.New(responsePtrType).Interface().(U)
		go func() {
			// Run the hook function.
			f(ctx, request, hookResponse)
			// Send the response to the response channel.
			responseChan <- hookResponse
			// Decrement the wait group counter.
			wg.Done()
		}()
	}

	// Wait for all hooks to finish and close the response channel. Closing the response channel
	// signals that all hooks have finished and will end the range loop that reads from the response
	// channel below.
	go func() {
		wg.Wait()
		close(responseChan)
	}()

	// Read from the response channel and collect all individualResponses into a slice.
	individualResponses := make([]U, 0, len(hookFuncs))
	for r := range responseChan {
		individualResponses = append(individualResponses, r)
	}

	// Aggregate all responses into a single response.
	aggregateResponses(response, individualResponses)
}

// aggregateResponses aggregates all responses into a single response.
func aggregateResponses[T runtimehooksv1.ResponseObject](
	aggregatedResponse T,
	responses []T,
) {
	var (
		// Initialize slices to store success and failure messages.
		successMessages, failureMessages []string
		// Initialize retryAfterSeconds to 0.
		retryAfterSeconds int32
		// Default to success. If any response is a failure, the aggregated response will be a failure.
		aggregatedResponseStatus = runtimehooksv1.ResponseStatusSuccess
	)

	// Iterate over all responses and aggregate the responses.
	for _, resp := range responses {
		switch resp.GetStatus() {
		// If the response status is failure, set the aggregated response status to failure and append
		// the message to the failure messages slice.
		// If the response is a RetryResponseObject, set the retryAfterSeconds to the lowest non-zero
		// value between the current retryAfterSeconds and the retryAfterSeconds of the response.
		case runtimehooksv1.ResponseStatusFailure:
			aggregatedResponseStatus = runtimehooksv1.ResponseStatusFailure

			// Only append the message if it is not empty.
			if resp.GetMessage() != "" {
				failureMessages = append(failureMessages, resp.GetMessage())
			}

			retryResp, ok := any(resp).(runtimehooksv1.RetryResponseObject)
			if ok {
				retryAfterSeconds = util.LowestNonZeroInt32(
					retryAfterSeconds,
					retryResp.GetRetryAfterSeconds(),
				)
			}
		// If the response status is success, append the message to the success messages slice.
		case runtimehooksv1.ResponseStatusSuccess:
			// Only append the message if it is not empty.
			if resp.GetMessage() != "" {
				successMessages = append(successMessages, resp.GetMessage())
			}
		}
	}

	// Set the aggregated response status.
	aggregatedResponse.SetStatus(aggregatedResponseStatus)

	switch aggregatedResponse.GetStatus() {
	// If the aggregated response status is failure, set the message to the failure messages
	// concatenated with a comma, and set the retryAfterSeconds if it is greater than 0.
	case runtimehooksv1.ResponseStatusFailure:
		aggregatedResponse.SetMessage(strings.Join(failureMessages, ", "))

		if retryAfterSeconds > 0 {
			// If retryAfterSeconds is set, we can safely assume that the response is a RetryResponseObject.
			any(aggregatedResponse).(runtimehooksv1.RetryResponseObject).SetRetryAfterSeconds(
				retryAfterSeconds,
			)
		}

	// If the aggregated response status is success, set the message to the success messages
	// concatenated with a comma.
	case runtimehooksv1.ResponseStatusSuccess:
		aggregatedResponse.SetMessage(strings.Join(successMessages, ", "))
	}
}

type parallelBCC struct {
	name string

	hooks []BeforeClusterCreate
}

func (p *parallelBCC) BeforeClusterCreate(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterCreateRequest,
	resp *runtimehooksv1.BeforeClusterCreateResponse,
) {
	hookFuncs := lo.Map(
		p.hooks,
		func(h BeforeClusterCreate, _ int) func(
			context.Context,
			*runtimehooksv1.BeforeClusterCreateRequest,
			*runtimehooksv1.BeforeClusterCreateResponse,
		) {
			return h.BeforeClusterCreate
		},
	)

	runHooksInParallel(ctx, hookFuncs, req, resp)
}

func (p *parallelBCC) Name() string {
	return p.name
}

func ParallelBeforeClusterCreateHook(
	name string,
	hooks ...BeforeClusterCreate,
) NamedBeforeClusterCreate {
	return &parallelBCC{
		name:  name,
		hooks: hooks,
	}
}

type parallelACPI struct {
	name  string
	hooks []AfterControlPlaneInitialized
}

func (p *parallelACPI) AfterControlPlaneInitialized(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneInitializedRequest,
	resp *runtimehooksv1.AfterControlPlaneInitializedResponse,
) {
	hookFuncs := lo.Map(
		p.hooks,
		func(h AfterControlPlaneInitialized, _ int) func(
			context.Context,
			*runtimehooksv1.AfterControlPlaneInitializedRequest,
			*runtimehooksv1.AfterControlPlaneInitializedResponse,
		) {
			return h.AfterControlPlaneInitialized
		},
	)

	runHooksInParallel(ctx, hookFuncs, req, resp)
}

func (p *parallelACPI) Name() string {
	return p.name
}

func ParallelAfterControlPlaneInitializedHook(
	name string, hooks ...AfterControlPlaneInitialized,
) NamedAfterControlPlaneInitialized {
	return &parallelACPI{
		name:  name,
		hooks: hooks,
	}
}

type parallelBCU struct {
	name string

	hooks []BeforeClusterUpgrade
}

func (p *parallelBCU) BeforeClusterUpgrade(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterUpgradeRequest,
	resp *runtimehooksv1.BeforeClusterUpgradeResponse,
) {
	hookFuncs := lo.Map(
		p.hooks,
		func(h BeforeClusterUpgrade, _ int) func(
			context.Context,
			*runtimehooksv1.BeforeClusterUpgradeRequest,
			*runtimehooksv1.BeforeClusterUpgradeResponse,
		) {
			return h.BeforeClusterUpgrade
		},
	)

	runHooksInParallel(ctx, hookFuncs, req, resp)
}

func (p *parallelBCU) Name() string {
	return p.name
}

func ParallelBeforeClusterUpgradeHook(
	name string,
	hooks ...BeforeClusterUpgrade,
) NamedBeforeClusterUpgrade {
	return &parallelBCU{
		name:  name,
		hooks: hooks,
	}
}

type parallelACPU struct {
	name string

	hooks []AfterControlPlaneUpgrade
}

func (p *parallelACPU) AfterControlPlaneUpgrade(
	ctx context.Context,
	req *runtimehooksv1.AfterControlPlaneUpgradeRequest,
	resp *runtimehooksv1.AfterControlPlaneUpgradeResponse,
) {
	hookFuncs := lo.Map(
		p.hooks,
		func(h AfterControlPlaneUpgrade, _ int) func(
			context.Context,
			*runtimehooksv1.AfterControlPlaneUpgradeRequest,
			*runtimehooksv1.AfterControlPlaneUpgradeResponse,
		) {
			return h.AfterControlPlaneUpgrade
		},
	)

	runHooksInParallel(ctx, hookFuncs, req, resp)
}

func (p *parallelACPU) Name() string {
	return p.name
}

func ParallelAfterControlPlaneUpgradeHook(
	name string,
	hooks ...AfterControlPlaneUpgrade,
) NamedAfterControlPlaneUpgrade {
	return &parallelACPU{
		name:  name,
		hooks: hooks,
	}
}

type parallelBCD struct {
	name string

	hooks []BeforeClusterDelete
}

func (p *parallelBCD) BeforeClusterDelete(
	ctx context.Context,
	req *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	hookFuncs := lo.Map(
		p.hooks,
		func(h BeforeClusterDelete, _ int) func(
			context.Context,
			*runtimehooksv1.BeforeClusterDeleteRequest,
			*runtimehooksv1.BeforeClusterDeleteResponse,
		) {
			return h.BeforeClusterDelete
		},
	)

	runHooksInParallel(ctx, hookFuncs, req, resp)
}

func (p *parallelBCD) Name() string {
	return p.name
}

func ParallelBeforeClusterDeleteHook(
	name string,
	hooks ...BeforeClusterDelete,
) NamedBeforeClusterDelete {
	return &parallelBCD{
		name:  name,
		hooks: hooks,
	}
}
