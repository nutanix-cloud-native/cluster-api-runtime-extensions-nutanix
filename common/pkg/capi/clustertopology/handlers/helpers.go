// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ClusterKeyFromReq(req *runtimehooksv1.GeneratePatchesRequest) client.ObjectKey {
	for i := range req.Items {
		item := req.Items[i]
		if item.HolderReference.Kind == "Cluster" &&
			(item.HolderReference.APIVersion == clusterv1beta1.GroupVersion.String() ||
				item.HolderReference.APIVersion == clusterv1.GroupVersion.String()) {
			return client.ObjectKey{
				Namespace: item.HolderReference.Namespace,
				Name:      item.HolderReference.Name,
			}
		}
	}

	return client.ObjectKey{}
}

// GetHolderObject fetches the object identified by holderRef as an Unstructured.
func GetHolderObject(
	ctx context.Context,
	c client.Client,
	holderRef runtimehooksv1.HolderReference,
) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(holderRef.APIVersion)
	obj.SetKind(holderRef.Kind)
	if err := c.Get(ctx, client.ObjectKey{
		Name:      holderRef.Name,
		Namespace: holderRef.Namespace,
	}, obj); err != nil {
		return nil, fmt.Errorf("failed to get holder %s %s/%s: %w",
			holderRef.Kind, holderRef.Namespace, holderRef.Name, err)
	}
	return obj, nil
}
