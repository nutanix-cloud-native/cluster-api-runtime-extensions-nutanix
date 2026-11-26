// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package multus provides a lifecycle handler for deploying the Multus meta-CNI addon.
//
// MultusHandler implements AfterControlPlaneInitialized and BeforeClusterUpgrade hooks:
//   - Deploys Multus via HelmAddon when explicitly enabled at clusterConfig.addons.cni.multus
//   - Reconciles pre-existing multus-<cluster-uuid> HelmChartProxy resources during upgrades
//     even when clusterConfig.addons.cni.multus is unset, preserving backwards compatibility
//     without mutating Cluster.spec
//   - Populates daemonConfig.readinessIndicatorFile with the primary CNI readiness socket path
//     (via cni.ReadinessSocketPath) unless explicitly overridden in custom Helm values.
package multus
