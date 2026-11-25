// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
)

func Test_runHooksInParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                       string
		individualResponses        []*runtimehooksv1.CommonRetryResponse
		expectedAggregatedResponse *runtimehooksv1.CommonRetryResponse
	}{{
		name:                "No responses",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		},
	}, {
		name: "Single success response",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		},
	}, {
		name: "Single failure response",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		},
	}, {
		name: "Success followed by failure",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		},
	}, {
		name: "Failure followed by success",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		},
	}, {
		name: "Multiple failures",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
			},
		},
	}, {
		name: "Multiple successes",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		},
	}, {
		name: "Multiple successes with messages",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "first success",
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "second success",
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "first success, second success",
			},
		},
	}, {
		name: "Multiple failures with messages",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "first failure",
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "second failure",
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "first failure, second failure",
			},
		},
	}, {
		name: "Failure followed by success with messages",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "failure",
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "success",
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "failure",
			},
		},
	}, {
		name: "Success followed by failure with messages",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "success",
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "failure",
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "failure",
			},
		},
	}, {
		name: "Success followed by failures with messages and single retry after",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "success",
			},
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "failure",
			},
			RetryAfterSeconds: 10,
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "another failure",
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "failure, another failure",
			},
			RetryAfterSeconds: 10,
		},
	}, {
		name: "Single blocking success response preserves retry after",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "cleanup in progress",
			},
			RetryAfterSeconds: 5,
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "cleanup in progress",
			},
			RetryAfterSeconds: 5,
		},
	}, {
		name: "Multiple success responses pick lowest non-zero retry after",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "waiting on lb",
			},
			RetryAfterSeconds: 10,
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
			RetryAfterSeconds: 0,
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "waiting on konnector",
			},
			RetryAfterSeconds: 5,
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "waiting on lb, waiting on konnector",
			},
			RetryAfterSeconds: 5,
		},
	}, {
		name: "Blocking success combined with failure without retry after does not leak success retry after",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "waiting on konnector",
			},
			RetryAfterSeconds: 5,
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "fatal error",
			},
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "fatal error",
			},
			RetryAfterSeconds: 0,
		},
	}, {
		name: "Blocking success combined with failure with retry after uses failure retry after",
		individualResponses: []*runtimehooksv1.CommonRetryResponse{{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "waiting on konnector",
			},
			RetryAfterSeconds: 3,
		}, {
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "transient error",
			},
			RetryAfterSeconds: 10,
		}},
		expectedAggregatedResponse: &runtimehooksv1.CommonRetryResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "transient error",
			},
			RetryAfterSeconds: 10,
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			hookFuncs := make(
				[]func(
					context.Context,
					*runtimehooksv1.BeforeClusterUpgradeRequest,
					*runtimehooksv1.BeforeClusterUpgradeResponse,
				),
				0, len(tt.individualResponses),
			)
			for _, individualResponse := range tt.individualResponses {
				hookFuncs = append(hookFuncs, func(
					ctx context.Context,
					_ *runtimehooksv1.BeforeClusterUpgradeRequest,
					resp *runtimehooksv1.BeforeClusterUpgradeResponse,
				) {
					resp.SetStatus(individualResponse.GetStatus())
					resp.SetMessage(individualResponse.GetMessage())
					resp.SetRetryAfterSeconds(individualResponse.GetRetryAfterSeconds())
				})
			}

			gotResponse := &runtimehooksv1.BeforeClusterUpgradeResponse{}
			runHooksInParallel(
				context.Background(),
				hookFuncs,
				&runtimehooksv1.BeforeClusterUpgradeRequest{},
				gotResponse,
			)

			g.Expect(gotResponse.GetStatus()).
				To(gomega.Equal(tt.expectedAggregatedResponse.GetStatus()))
			g.Expect(gotResponse.GetRetryAfterSeconds()).
				To(gomega.Equal(tt.expectedAggregatedResponse.GetRetryAfterSeconds()))

			// As we call funcs in parallel, response messages could be in any order so let's split the string and
			// compare using regex.
			gotResponseMessage := gotResponse.GetMessage()
			for expectedMessagePart := range strings.SplitSeq(tt.expectedAggregatedResponse.GetMessage(), ", ") {
				g.Expect(gotResponseMessage).To(
					gomega.MatchRegexp(
						fmt.Sprintf("((^|, )%s(, |$))", expectedMessagePart),
					),
				)
			}
		})
	}
}

type stubBCDHandler struct {
	setStatus         bool
	status            runtimehooksv1.ResponseStatus
	message           string
	retryAfterSeconds int32
}

func (s stubBCDHandler) BeforeClusterDelete(
	_ context.Context,
	_ *runtimehooksv1.BeforeClusterDeleteRequest,
	resp *runtimehooksv1.BeforeClusterDeleteResponse,
) {
	if s.setStatus {
		resp.SetStatus(s.status)
	}
	if s.message != "" {
		resp.SetMessage(s.message)
	}
	if s.retryAfterSeconds != 0 {
		resp.SetRetryAfterSeconds(s.retryAfterSeconds)
	}
}

func TestParallelBeforeClusterDeleteHook_DefaultsStatusToSuccessAndPreservesRetryAfter(t *testing.T) {
	t.Parallel()

	hook := ParallelBeforeClusterDeleteHook(
		"caren",
		// Simulates a handler (like RegistryHandler when addon is disabled) that returns early
		// without calling resp.SetStatus, relying on CAPI's default ResponseStatusSuccess.
		stubBCDHandler{message: "addon skipped"},
		stubBCDHandler{
			setStatus:         true,
			status:            runtimehooksv1.ResponseStatusSuccess,
			message:           "cleanup in progress",
			retryAfterSeconds: 5,
		},
	)

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	hook.BeforeClusterDelete(context.Background(), &runtimehooksv1.BeforeClusterDeleteRequest{}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.GetStatus())
	assert.Equal(t, int32(5), resp.GetRetryAfterSeconds())
	assert.Contains(t, resp.GetMessage(), "addon skipped")
	assert.Contains(t, resp.GetMessage(), "cleanup in progress")
}
