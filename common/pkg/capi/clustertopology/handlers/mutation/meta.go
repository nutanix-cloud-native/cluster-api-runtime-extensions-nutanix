// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"context"
	"fmt"
	"sync"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/cluster-api/exp/runtime/topologymutation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
)

type MutateFunc func(
	ctx context.Context,
	obj *unstructured.Unstructured,
	vars map[string]apiextensionsv1.JSON,
	holderRef runtimehooksv1.HolderReference,
	clusterKey client.ObjectKey,
) error

type ClusterGetter func(context.Context) (*clusterv1.Cluster, error)

type MetaMutator interface {
	Mutate(
		ctx context.Context,
		obj *unstructured.Unstructured,
		vars map[string]apiextensionsv1.JSON,
		holderRef runtimehooksv1.HolderReference,
		clusterKey client.ObjectKey,
		getCluster ClusterGetter,
	) error
}

type metaGeneratePatches struct {
	name     string
	mutators []MetaMutator
	cl       client.Client
}

func NewMetaGeneratePatchesHandler(
	name string,
	cl client.Client,
	mutators ...MetaMutator,
) handlers.Named {
	return metaGeneratePatches{
		name:     name,
		cl:       cl,
		mutators: mutators,
	}
}

func (mgp metaGeneratePatches) Name() string {
	return mgp.name
}

func (mgp metaGeneratePatches) CreateClusterGetter(
	clusterKey client.ObjectKey,
) func(context.Context) (*clusterv1.Cluster, error) {
	return func(ctx context.Context) (*clusterv1.Cluster, error) {
		var (
			cluster clusterv1.Cluster
			err     error
			once    sync.Once
		)
		once.Do(func() {
			err = mgp.cl.Get(ctx, clusterKey, &cluster)
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch cluster: %w", err)
		}
		return &cluster, nil
	}
}

func (mgp metaGeneratePatches) GeneratePatches(
	ctx context.Context,
	req *runtimehooksv1.GeneratePatchesRequest,
	resp *runtimehooksv1.GeneratePatchesResponse,
) {
	clusterKey := handlers.ClusterKeyFromReq(req)
	clusterGetter := mgp.CreateClusterGetter(clusterKey)
	topologymutation.WalkTemplates(
		ctx,
		unstructured.UnstructuredJSONScheme,
		req,
		resp,
		func(
			ctx context.Context,
			obj runtime.Object,
			vars map[string]apiextensionsv1.JSON,
			holderRef runtimehooksv1.HolderReference,
		) error {
			for _, h := range mgp.mutators {
				if err := h.Mutate(ctx, obj.(*unstructured.Unstructured), vars, holderRef, clusterKey, clusterGetter); err != nil {
					return err
				}
			}

			return nil
		},
	)
}
