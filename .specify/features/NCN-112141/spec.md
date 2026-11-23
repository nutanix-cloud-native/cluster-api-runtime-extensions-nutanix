<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Reconcile MetalLB configuration in a dedicated controller

**Jira Ticket**: [NCN-112141](https://jira.nutanix.com/browse/NCN-112141), [NCN-111113](https://jira.nutanix.com/browse/NCN-111113)
**Feature Branch**: `jimmi/metallb-config-controller-5091`
**Created**: 2026-11-23
**Status**: Approved

## User Scenarios & Testing

### User Story 1 - Fast, non-blocking `AfterControlPlaneInitialized` and `BeforeClusterUpgrade` hooks (Priority: P1)

When Cluster API invokes `AfterControlPlaneInitialized` or `BeforeClusterUpgrade` on a Cluster configured with `addons.serviceLoadBalancer.provider: MetalLB`, CAREN ensures the `metallb-system` namespace on the workload cluster and applies the `metallb` `HelmChartProxy` on the management cluster without blocking on Helm chart deployment or workload webhook readiness.

**Why this priority**: Waiting for CAAPH to install the MetalLB Helm chart and polling `IPAddressPool`/`L2Advertisement` inside the synchronous runtime extension HTTP hook exceeds Cluster API's 10s–30s hook timeout, causing CAPI exponential backoff (up to 16m40s) and leaving `IPAddressPool` unapplied for minutes after control plane initialization.

**Acceptance Scenarios**:

1. **Given** a Cluster with `provider: MetalLB` and `configuration.addressRanges`, **When** `AfterControlPlaneInitialized` or `BeforeClusterUpgrade` runs, **Then** the `metallb-system` namespace (with privileged PodSecurity labels) and `metallb-<cluster-uuid>` `HelmChartProxy` are applied and the hook returns `Success` immediately without waiting for `HelmChartProxy` `Ready=True`.

---

### User Story 2 - Asynchronous reconciliation of `IPAddressPool` and `L2Advertisement` (Priority: P1)

Once CAAPH marks the `metallb-<cluster-uuid>` `HelmChartProxy` `Ready=True` for the current generation, CAREN's controller reconciles the `metallb` `IPAddressPool` and `L2Advertisement` onto the workload cluster via Server-Side Apply with strict field validation, and keeps them updated if `addressRanges` changes on the `Cluster`.

**Acceptance Scenarios**:

1. **Given** a Cluster with `provider: MetalLB` and `configuration.addressRanges`, **When** the `metallb-<cluster-uuid>` `HelmChartProxy` transitions to `Ready=True`, **Then** `IPAddressPool` and `L2Advertisement` named `metallb` are reconciled in `metallb-system` on the workload cluster.
2. **Given** `HelmChartProxy` is not yet created or not `Ready=True` for its current generation, **When** the controller reconciles the Cluster, **Then** it exits cleanly without error and reconciles immediately when the `HelmChartProxy` watch fires.
3. **Given** `addressRanges` is updated on `Cluster.spec.topology.variables`, **When** the `Cluster` watch fires, **Then** the workload cluster's `IPAddressPool` is updated to match.
4. **Given** `HelmChartProxy` is `Ready=True` before the workload cluster's `metallb-controller` validation webhook endpoints are ready (for example while worker nodes and CNI finish starting), **When** `ApplyConfiguration` transiently fails repeatedly, **Then** workqueue retry backoff is capped at 10s so configuration is reconciled promptly once the webhook becomes available rather than backing off up to 16m40s.
5. **Given** `IPAddressPool` or `L2Advertisement` was modified by a conflicting field manager on the workload cluster, **When** the controller reconciles, **Then** it returns an actionable conflict error message identifying the resource and required configuration.

## Requirements

- **FR-001**: Remove `.WithDefaultWaiter()` and the `IPAddressPool`/`L2Advertisement` SSA polling loop from `MetalLB.Apply` in `pkg/handlers/lifecycle/serviceloadbalancer/metallb/handler.go`.
- **FR-002**: Extract reusable `ApplyConfiguration` and `HelmChartProxyName` helpers in `pkg/handlers/lifecycle/serviceloadbalancer/metallb/configuration.go`, preserving `FieldValidationStrict` and existing conflict error messages.
- **FR-003**: Implement `pkg/controllers/metallbconfig.Reconciler` watching `Cluster` and MetalLB `HelmChartProxy` resources (supporting non-controller `OwnerReference` and `ClusterSelector` fallback), with workqueue retry delay capped at 10s so transient webhook unavailability never stalls reconciliation for minutes.
- **FR-004**: Skip reconciliation for deleted, paused, non-topology, or non-MetalLB/unconfigured Clusters.
- **FR-005**: Wire `metallbconfig.Reconciler` and `--metallb-config-concurrency` in `cmd/main.go`.
- **FR-006**: Update `docs/content/addons/serviceloadbalancer.md` to document asynchronous configuration reconciliation.
- **FR-007**: Do not alter any topology mutation handler `GeneratePatches` output.
