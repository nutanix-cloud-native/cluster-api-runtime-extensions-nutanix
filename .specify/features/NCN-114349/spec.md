<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Privileged PSA label on ntnx-system and node-feature-discovery namespaces

**Jira Ticket**: [NCN-114349](https://jira.nutanix.com/browse/NCN-114349)
**Feature Branch**: `NCN-114349-psa-label-ntnx-system-nfd-namespaces`
**Created**: 2026-05-14
**Status**: Draft
**Input**: User description: "on main, we need to ensure that ntnx-system and node-feature-discovery namespaces are labelled with privileged PSA"

## User Scenarios & Testing

### User Story 1 - Operator enables baseline/restricted PSA enforcement cluster-wide (Priority: P1)

An operator follows the Dirty Frag mitigation guide and turns on
`clusterConfig.podSecurityAdmission.enforce: baseline` (or `restricted`) on a
workload cluster. They expect all CAREN-managed addon namespaces to keep
working without needing per-cluster manual labelling, because the addons
legitimately need privileged pod features (hostPath, hostNetwork, privileged
containers).

**Why this priority**: This is the explicit reason the change exists. Without
it, the very mitigation we ship guidance for breaks our own addons on upgrade.

**Independent Test**: On a workload cluster with CAREN-managed Nutanix CSI,
Konnector Agent, and NFD installed, set the cluster-wide PSA enforcement to
`baseline`. Verify that pods in `ntnx-system` and `node-feature-discovery`
continue to schedule, including after pod recreation. Verify the namespaces
carry the label `pod-security.kubernetes.io/enforce=privileged`.

**Scope note**: Only CAREN-managed addons that actually install into
`ntnx-system` are in scope. Survey of the codebase confirms these are
**Nutanix CSI** and **Konnector Agent**. The Nutanix CCM handler installs into
`kube-system` and the Nutanix Flow CNI handler installs into `kube-system`
(with image-pull secrets copied to `flow-cni-system` and `ovn-kubernetes`),
so neither needs `ntnx-system` labelling. `kube-system` is the default PSA
exemption and is documented as such in the Dirty Frag guide, so it does not
need labelling either.

**Acceptance Scenarios**:

1. **Given** a CAREN-managed workload cluster with the Nutanix CSI lifecycle
   handler enabled, **When** the handler has run to completion, **Then** the
   `ntnx-system` namespace on the workload cluster exists with the label
   `pod-security.kubernetes.io/enforce=privileged`.
2. **Given** a CAREN-managed workload cluster with the Konnector Agent
   lifecycle handler enabled, **When** the handler has run to completion,
   **Then** the `ntnx-system` namespace on the workload cluster exists with
   the label `pod-security.kubernetes.io/enforce=privileged`.
3. **Given** a CAREN-managed workload cluster with NFD deployed via the
   `HelmAddon` strategy, **When** the NFD lifecycle handler has run to
   completion, **Then** the `node-feature-discovery` namespace on the workload
   cluster exists with the label
   `pod-security.kubernetes.io/enforce=privileged`.
4. **Given** the `node-feature-discovery` namespace already exists on the
   workload cluster without PSA labels (created by an older CAREN, by a stuck
   CAAPH, or by the user), **When** the NFD lifecycle handler runs,
   **Then** the label is added to the existing namespace without conflict.

### User Story 2 - Operator upgrades CAREN on a working cluster (Priority: P1)

An operator upgrades CAREN on a management cluster that has many running
workload clusters. They expect no churn on existing clusters and no impact
beyond namespace metadata changes.

**Why this priority**: Constitution principle VII (no silent rollouts). This
change must not bump any topology mutation handler version and must not
generate a Machine rollout.

**Independent Test**: Compare the output of every registered topology mutation
handler's `GeneratePatches` before and after the change for a representative
Cluster. The output must be byte-identical.

**Acceptance Scenarios**:

1. **Given** an existing managed Cluster on the previous CAREN version,
   **When** CAREN is upgraded to the version including this change, **Then**
   no MachineDeployment or KubeadmControlPlane is rolled out as a result of
   the upgrade.

### Edge Cases

- The namespace already exists on the workload cluster with a different value
  for `pod-security.kubernetes.io/enforce` (e.g. `baseline`, set by an
  operator). The handler must overwrite the value back to `privileged`
  because the addons cannot run otherwise. This is consistent with the
  MetalLB precedent in the same codebase.
- The namespace already carries the older two-label form (`enforce`
  + `enforce-version: latest`). The handler must not remove the
  `enforce-version` label (out of scope for this change). The label set the
  handler manages is "the labels it explicitly applies".
- The workload cluster API is unreachable when the handler runs. The handler
  must return an error so CAPI retries — consistent with current behaviour of
  `CopySecretToRemoteCluster`.
- Multiple Nutanix lifecycle handlers (CSI + CCM + Konnector + Flow CNI) run
  on the same cluster and all try to ensure the same `ntnx-system` namespace.
  Server-side apply with a stable field-manager makes this safe and
  idempotent.

## Requirements

### Functional Requirements

- **FR-001**: The Nutanix CSI lifecycle handler MUST ensure the
  `ntnx-system` namespace exists on the workload cluster with the label
  `pod-security.kubernetes.io/enforce=privileged` before installing the
  CSI Helm chart.
- **FR-002**: The Konnector Agent lifecycle handler MUST ensure the
  `ntnx-system` namespace exists on the workload cluster with the label
  `pod-security.kubernetes.io/enforce=privileged` before installing the
  Konnector Agent Helm chart.
- **FR-003**: The NFD lifecycle handler, when using the `HelmAddon`
  strategy, MUST ensure the `node-feature-discovery` namespace exists on the
  workload cluster with the label
  `pod-security.kubernetes.io/enforce=privileged` before installing the
  NFD Helm chart.
- **FR-004**: Label application MUST use server-side apply so that repeated
  reconciles are idempotent and the handler's field-manager owns only the
  label keys it explicitly sets.
- **FR-005**: The change MUST NOT modify the output of any topology mutation
  handler's `GeneratePatches`, and MUST NOT require a handler version bump.
- **FR-006**: The handler MUST NOT set
  `pod-security.kubernetes.io/enforce-version` explicitly, because PSA
  defaults the version to `latest` when omitted.

### Key Entities

- **Namespace `ntnx-system`** (workload cluster): Holds Nutanix CSI and
  Konnector Agent workloads. Requires PSA `privileged`.
- **Namespace `node-feature-discovery`** (workload cluster): Holds the NFD
  master/worker DaemonSets. Requires PSA `privileged`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: On a workload cluster after the relevant lifecycle handler has
  reconciled, `kubectl get namespace ntnx-system -o
  jsonpath='{.metadata.labels.pod-security\.kubernetes\.io/enforce}'` returns
  `privileged`. Same check for `node-feature-discovery` when NFD is enabled.
- **SC-002**: After enabling
  `clusterConfig.podSecurityAdmission.enforce: baseline` cluster-wide on a
  workload cluster with the affected addons installed, zero new pods in
  `ntnx-system` or `node-feature-discovery` are rejected at admission for PSA
  violations.
- **SC-003**: Upgrading CAREN from the previous release to the release
  including this change on a management cluster with N workload clusters
  triggers zero MachineDeployment or KubeadmControlPlane rollouts attributable
  to this change.
