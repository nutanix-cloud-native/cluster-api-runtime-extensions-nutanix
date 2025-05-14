// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package registry provides a handler for managing a registry addon in clusters.
// The clusters will also be configured to use the registry as a Containerd mirror (done in a different handler).
// +kubebuilder:rbac:groups="cert-manager.io",resources=clusterissuers,verbs=create;patch;update
// +kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=list;get;create;patch;update
package registry
