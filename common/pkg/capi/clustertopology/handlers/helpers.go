// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ClusterKeyFromReq(req *runtimehooksv1.GeneratePatchesRequest) client.ObjectKey {
	for i := range req.Items {
		item := req.Items[i]
		if item.HolderReference.Kind == "Cluster" &&
			item.HolderReference.APIVersion == clusterv1.GroupVersion.String() {
			return client.ObjectKey{
				Namespace: item.HolderReference.Namespace,
				Name:      item.HolderReference.Name,
			}
		}
	}

	return client.ObjectKey{}
}
