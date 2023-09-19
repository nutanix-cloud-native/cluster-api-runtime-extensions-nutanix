// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"errors"

	capiv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/exp/runtime/hooks/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// clusterKeyContextKey is how we find a cluster key from a context.Context.
type clusterKeyContextKey struct{}

func ClusterKeyFrom(ctx context.Context) (client.ObjectKey, error) {
	clusterKey, ok := ctx.Value(clusterKeyContextKey{}).(client.ObjectKey)
	if !ok || clusterKey.Name == "" {
		return client.ObjectKey{}, errors.New(
			"failed to detect cluster name from GeneratePatch request",
		)
	}

	return clusterKey, nil
}

func ClusterKeyInto(
	ctx context.Context, req *runtimehooksv1.GeneratePatchesRequest,
) context.Context {
	clusterKey := client.ObjectKey{}

	for i := range req.Items {
		item := req.Items[i]
		if item.HolderReference.Kind == "Cluster" &&
			item.HolderReference.APIVersion == capiv1.GroupVersion.String() {
			clusterKey.Name = item.HolderReference.Name
			clusterKey.Namespace = item.HolderReference.Namespace
		}
	}

	if clusterKey.Name != "" {
		return context.WithValue(ctx, clusterKeyContextKey{}, clusterKey)
	}

	return ctx
}
