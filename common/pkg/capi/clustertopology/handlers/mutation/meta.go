// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"strings"

	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
)

type metaGeneratePatches struct {
	name            string
	wrappedHandlers []GeneratePatches
}

func NewMetaGeneratePatchesHandler(name string, gp ...GeneratePatches) handlers.Named {
	return metaGeneratePatches{
		name:            name,
		wrappedHandlers: gp,
	}
}

func (mgp metaGeneratePatches) Name() string {
	return mgp.name
}

func (mgp metaGeneratePatches) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	for _, h := range mgp.wrappedHandlers {
		wrappedResp := &runtimehooksv1.GeneratePatchesResponse{}
		h.GeneratePatches(ctx, req, wrappedResp)
		resp.Items = append(resp.Items, wrappedResp.Items...)
		if wrappedResp.Message != "" {
			resp.Message = strings.TrimPrefix(resp.Message+"\n"+wrappedResp.Message, "\n")
		}
		resp.Status = wrappedResp.Status
		if resp.Status == runtimehooksv1.ResponseStatusFailure {
			return
		}
	}
}
