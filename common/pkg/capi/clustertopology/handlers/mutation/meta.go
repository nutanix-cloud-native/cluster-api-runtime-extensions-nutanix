// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
)

type MetaMutater interface {
	Mutate(
		ctx context.Context,
		obj runtime.Object,
		vars map[string]apiextensionsv1.JSON,
		holderRef runtimehooksv1.HolderReference,
		clusterKey client.ObjectKey,
	) error
}

type metaGeneratePatches struct {
	name     string
	decoder  runtime.Decoder
	mutaters []MetaMutater
}

func NewMetaGeneratePatchesHandler(name string, m ...MetaMutater) handlers.Named {
	scheme := runtime.NewScheme()
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	return metaGeneratePatches{
		name: name,
		decoder: serializer.NewCodecFactory(scheme).UniversalDecoder(
			controlplanev1.GroupVersion,
			bootstrapv1.GroupVersion,
		),
		mutaters: m,
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
	clusterKey := handlers.ClusterKeyFromReq(req)

	topologymutation.WalkTemplates(
		ctx,
		mgp.decoder,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			for _, h := range mgp.mutaters {
				if err := h.Mutate(ctx, obj, vars, holderRef, clusterKey); err != nil {
					return err
				}
			}

			return nil
		},
	)
}
