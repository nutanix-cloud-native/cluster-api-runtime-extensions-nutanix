// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
)

type testHandler struct {
	resp *runtimehooksv1.GeneratePatchesResponse
}

var _ GeneratePatches = &testHandler{}

func (h *testHandler) GeneratePatches(
	_ context.Context,
	_ *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	resp.Items = append(resp.Items, h.resp.Items...)
	resp.Message = h.resp.Message
	resp.Status = h.resp.Status
}

func TestMetaGeneratePatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		wrappedHandlers  []GeneratePatches
		expectedResponse *runtimehooksv1.GeneratePatchesResponse
	}{{
		name: "no handlers",
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
			},
		},
	}, {
		name: "single success handler",
		wrappedHandlers: []GeneratePatches{
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusSuccess,
						Message: "This is a success",
					},
					Items: []runtimehooksv1.GeneratePatchesResponseItem{{
						UID:       "1234",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is a patch`),
					}},
				},
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusSuccess,
				Message: "This is a success",
			},
			Items: []runtimehooksv1.GeneratePatchesResponseItem{{
				UID:       "1234",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     []byte(`this is a patch`),
			}},
		},
	}, {
		name: "single failure handler",
		wrappedHandlers: []GeneratePatches{
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusFailure,
						Message: "This is a failure",
					},
				},
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: "This is a failure",
			},
		},
	}, {
		name: "multiple success handlers",
		wrappedHandlers: []GeneratePatches{
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusSuccess,
						Message: "This is a success",
					},
					Items: []runtimehooksv1.GeneratePatchesResponseItem{{
						UID:       "1234",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is a patch`),
					}, {
						UID:       "12345",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is another patch`),
					}},
				},
			},
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusSuccess,
						Message: "This is also a success",
					},
					Items: []runtimehooksv1.GeneratePatchesResponseItem{{
						UID:       "123456",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is also a patch`),
					}},
				},
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusSuccess,
				Message: `This is a success
This is also a success`,
			},
			Items: []runtimehooksv1.GeneratePatchesResponseItem{{
				UID:       "1234",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     []byte(`this is a patch`),
			}, {
				UID:       "12345",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     []byte(`this is another patch`),
			}, {
				UID:       "123456",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     []byte(`this is also a patch`),
			}},
		},
	}, {
		name: "success handler followed by failure handler",
		wrappedHandlers: []GeneratePatches{
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusSuccess,
						Message: "This is a success",
					},
					Items: []runtimehooksv1.GeneratePatchesResponseItem{{
						UID:       "1234",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is a patch`),
					}, {
						UID:       "12345",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is another patch`),
					}},
				},
			},
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusFailure,
						Message: "This is a failure",
					},
				},
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status: runtimehooksv1.ResponseStatusFailure,
				Message: `This is a success
This is a failure`,
			},
			Items: []runtimehooksv1.GeneratePatchesResponseItem{{
				UID:       "1234",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     []byte(`this is a patch`),
			}, {
				UID:       "12345",
				PatchType: runtimehooksv1.JSONPatchType,
				Patch:     []byte(`this is another patch`),
			}},
		},
	}, {
		name: "failure handler followed by success handler",
		wrappedHandlers: []GeneratePatches{
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusFailure,
						Message: "This is a failure",
					},
				},
			},
			&testHandler{
				resp: &runtimehooksv1.GeneratePatchesResponse{
					CommonResponse: runtimehooksv1.CommonResponse{
						Status:  runtimehooksv1.ResponseStatusSuccess,
						Message: "This is a success",
					},
					Items: []runtimehooksv1.GeneratePatchesResponseItem{{
						UID:       "1234",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is a patch`),
					}, {
						UID:       "12345",
						PatchType: runtimehooksv1.JSONPatchType,
						Patch:     []byte(`this is another patch`),
					}},
				},
			},
		},
		expectedResponse: &runtimehooksv1.GeneratePatchesResponse{
			CommonResponse: runtimehooksv1.CommonResponse{
				Status:  runtimehooksv1.ResponseStatusFailure,
				Message: `This is a failure`,
			},
		},
	}}

	for idx := range tests {
		tt := tests[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			h := NewMetaGeneratePatchesHandler("", tt.wrappedHandlers...).(GeneratePatches)

			resp := &runtimehooksv1.GeneratePatchesResponse{}
			h.GeneratePatches(context.Background(), &runtimehooksv1.GeneratePatchesRequest{}, resp)

			g.Expect(resp).To(gomega.Equal(tt.expectedResponse))
		})
	}
}
