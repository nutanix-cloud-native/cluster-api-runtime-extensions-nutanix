// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package clusterautoscaler provides a handler for managing ClusterAutoscaler deployments on clusters
//
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=clusterresourcesets,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=watch;list;get
package clusterautoscaler
