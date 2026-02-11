// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capiannotations "sigs.k8s.io/cluster-api/util/annotations"
)

// IsPaused returns true if the Cluster is paused or the object has the `paused` annotation.
//
// This was copied from the CAPI annotations package to support deprecated v1beta1.
func IsPaused(cluster *clusterv1.Cluster, o metav1.Object) bool {
	if cluster.Spec.Paused {
		return true
	}
	return capiannotations.HasPaused(o)
}
