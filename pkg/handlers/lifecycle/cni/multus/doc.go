// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package multus provides a standalone lifecycle handler for Multus CNI that automatically
// deploys Multus when:
// - The cluster is on a supported cloud provider (EKS or Nutanix)
// - A supported CNI provider is configured (Cilium or Calico)
//
// MultusHandler implements the cluster lifecycle hooks and:
// - Detects the cloud provider from the cluster infrastructure
// - Reads CNI configuration from cluster variables
// - Gets the socket path for the configured CNI (via cni.SocketPath)
// - Automatically deploys Multus with socket-based configuration
//
// helmAddonStrategy is the internal strategy that handles:
// - Templating Helm values with the CNI socket path
// - Deploying Multus using HelmAddon strategy with Go template-based values
//
// Multus relies on the readinessIndicatorFile configuration to wait for the primary CNI
// to be ready, eliminating the need for explicit wait logic in the strategy.
//
// This package does NOT expose Multus in the API - it's an internal addon
// that deploys automatically based on cloud provider and CNI selection.
package multus
