// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
)

// ShouldSkipKubeProxy returns true if the cluster is configured to skip kube proxy installation.
func ShouldSkipKubeProxy(cluster *clusterv1.Cluster) bool {
	if cluster.Spec.Topology != nil {
		_, isSkipKubeProxy := cluster.Spec.Topology.ControlPlane.Metadata.Annotations[controlplanev1.SkipKubeProxyAnnotation]
		return isSkipKubeProxy
	}
	return false
}
