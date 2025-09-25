// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package nfd provides a handler for managing NFD deployments on clusters
//
// +kubebuilder:rbac:groups=addons.cluster.x-k8s.io,resources=clusterresourcesets,verbs=watch;list;get;create;patch;update;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=watch;list;get;create;patch;update;delete
package nfd
