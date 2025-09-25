// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cilium provides a handler for managing Cilium deployments on clusters, configurable via
// variables on the Cluster resource.
//
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=clusterresourcesets,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=watch;list;get;create;patch;update;delete
package cilium
