// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package nutanixflow provides a handler for managing Nutanix Flow Networking CNI deployments
// on clusters, configurable via variables on the Cluster resource.
//
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=helmchartproxies,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=watch;list;get;create;patch;update;delete
package nutanixflow
