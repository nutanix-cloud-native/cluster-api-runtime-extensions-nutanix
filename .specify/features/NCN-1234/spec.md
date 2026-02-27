<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Feature Specification: KubeletConfiguration Variable & Mutation Handler

**Jira Ticket**: NCN-1234
**Feature Branch**: `NCN-1234-kubelet-configuration`
**Created**: 2026-02-27
**Status**: Draft
**Input**: User description: "Create a variable and mutation handler that allows adding extra args to kubelet, with args derived from available kubelet flags to be as strongly typed as possible."

## User Scenarios & Testing

### User Story 1 — Set cluster-wide kubelet defaults (Priority: P1)

As a cluster operator, I want to set kubelet configuration defaults (e.g. `maxPods`,
`systemReserved`, `protectKernelDefaults`) at the cluster level so that all nodes
(control plane and workers) receive the same baseline configuration.

**Why this priority**: Most common use case — operators want a single place to configure
kubelet settings for the entire cluster.

**Independent Test**: Create a cluster with `clusterConfig.kubeletConfiguration.maxPods=200`
and verify the KubeletConfiguration patch file is present on both control plane and worker nodes.

**Acceptance Scenarios**:

1. **Given** a ClusterClass-based cluster with `clusterConfig.kubeletConfiguration.maxPods` set to 200,
   **When** the cluster is created,
   **Then** both `KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate` contain a
   KubeletConfiguration strategic merge patch file at
   `/etc/kubernetes/patches/kubeletconfiguration+strategic.json` with `maxPods: 200`.

2. **Given** a ClusterClass-based cluster with no `kubeletConfiguration` set,
   **When** the cluster is created,
   **Then** no kubelet configuration patch file is added (kubelet defaults apply).

3. **Given** a ClusterClass-based cluster with `clusterConfig.kubeletConfiguration.protectKernelDefaults`
   set to `true`,
   **When** the cluster is created,
   **Then** the patch file contains `protectKernelDefaults: true`.

---

### User Story 2 — Override kubelet settings per node group (Priority: P1)

As a cluster operator, I want to override kubelet settings for control plane or worker
nodes independently, so that I can tune node types differently (e.g. higher `maxPods`
on workers, stricter eviction on control plane).

**Why this priority**: Equally important as cluster-wide defaults — heterogeneous node
configuration is standard in production.

**Independent Test**: Create a cluster with cluster-level `maxPods=110` and worker-level
`maxPods=200`, verify workers get 200 and control plane gets 110.

**Acceptance Scenarios**:

1. **Given** a cluster with `clusterConfig.kubeletConfiguration.maxPods=110` and
   `workerConfig.kubeletConfiguration.maxPods=200`,
   **When** the cluster is created,
   **Then** the worker `KubeadmConfigTemplate` patch has `maxPods: 200` and the control
   plane `KubeadmControlPlaneTemplate` patch has `maxPods: 110`.

2. **Given** a cluster with only `clusterConfig.controlPlane.kubeletConfiguration.topologyManagerPolicy=restricted`
   (no cluster-level default),
   **When** the cluster is created,
   **Then** only the control plane template contains a patch with `topologyManagerPolicy: restricted`,
   and workers have no kubelet configuration patch.

3. **Given** a cluster with cluster-level `evictionHard` and worker-level `evictionHard`,
   **When** the cluster is created,
   **Then** the worker-level `evictionHard` fully replaces (not merges with) the cluster default.

---

### User Story 3 — Deprecate standalone maxParallelImagePullsPerNode (Priority: P2)

As a cluster operator using the existing `maxParallelImagePullsPerNode` field, I want
a smooth migration path to the new `kubeletConfiguration.maxParallelImagePulls` field
so that my existing clusters keep working.

**Why this priority**: Backwards compatibility. Existing users must not break.

**Independent Test**: Create a cluster using only the old field and verify it still works.
Then create one using both and verify the new field wins.

**Acceptance Scenarios**:

1. **Given** a cluster with only the deprecated `maxParallelImagePullsPerNode=4` set,
   **When** the cluster is created,
   **Then** the kubelet config patch contains `maxParallelImagePulls: 4` and
   `serializeImagePulls: false`.

2. **Given** a cluster with both `maxParallelImagePullsPerNode=4` and
   `kubeletConfiguration.maxParallelImagePulls=8`,
   **When** the cluster is created,
   **Then** the patch uses `maxParallelImagePulls: 8` (new field wins).

3. **Given** a cluster with both fields set to conflicting values,
   **When** the cluster spec is submitted,
   **Then** a validation warning is emitted indicating the deprecated field is ignored.

---

### User Story 4 — CRD-level validation rejects invalid input (Priority: P2)

As a cluster operator, I want invalid kubelet configuration to be rejected at
apply-time with clear error messages.

**Why this priority**: Prevents misconfiguration from reaching the kubelet.

**Independent Test**: Submit a cluster spec with `imageGCHighThresholdPercent=50` and
`imageGCLowThresholdPercent=80`, verify it is rejected.

**Acceptance Scenarios**:

1. **Given** a cluster spec with `imageGCHighThresholdPercent=50` and
   `imageGCLowThresholdPercent=80`,
   **When** the spec is submitted,
   **Then** it is rejected with message "imageGCHighThresholdPercent must be greater
   than imageGCLowThresholdPercent".

2. **Given** a cluster spec with `evictionSoftGracePeriod` set but `evictionSoft` not set,
   **When** the spec is submitted,
   **Then** it is rejected with message "evictionSoft must be set when
   evictionSoftGracePeriod is set".

3. **Given** a cluster spec with `systemReserved` containing key `"foo"`,
   **When** the spec is submitted,
   **Then** it is rejected because only `cpu`, `memory`, `ephemeral-storage`, `pid`
   are valid keys.

4. **Given** a cluster spec with `topologyManagerPolicy="invalid"`,
   **When** the spec is submitted,
   **Then** it is rejected by the enum validation.

---

### Edge Cases

- What happens when `kubeletConfiguration` is an empty object `{}`? No patch file should be generated.
- What happens when the cluster-level default and node-level override both set the same
  field? Node-level wins entirely (no deep merge of individual map entries within a field).
- What happens when `evictionSoftGracePeriod` has keys not present in `evictionSoft`?
  Rejected by CEL validation.
- What happens when `cpuManagerPolicy=static` but no CPU reservation is set in
  `systemReserved` or `kubeReserved`? Rejected by webhook validation with a clear message.
- What happens when the `maxParallelImagePulls` in the new struct conflicts with the
  value derived from the deprecated `maxParallelImagePullsPerNode`? New field takes
  precedence; webhook emits a warning.

## Requirements

### Functional Requirements

- **FR-001**: System MUST define a `KubeletConfiguration` API type with 17 strongly-typed
  fields matching a curated subset of the upstream `kubelet.config.k8s.io/v1beta1`
  `KubeletConfiguration`.
- **FR-002**: System MUST expose `KubeletConfiguration` at the cluster level via
  `KubeadmClusterConfigSpec.KubeletConfiguration`.
- **FR-003**: System MUST expose `KubeletConfiguration` at the node level via
  `KubeadmNodeSpec.KubeletConfiguration`, enabling per-control-plane and per-worker overrides.
- **FR-004**: System MUST implement a mutation handler that renders a KubeletConfiguration
  strategic merge patch file and injects it into `KubeadmControlPlaneTemplate` and
  `KubeadmConfigTemplate`.
- **FR-005**: Node-level overrides MUST take precedence over cluster-level defaults.
  Override semantics are per-field (not deep merge of map values within a field).
- **FR-006**: System MUST deprecate the existing standalone `maxParallelImagePullsPerNode`
  field on `KubeadmClusterConfigSpec`, keeping it functional with lower precedence than
  `KubeletConfiguration.MaxParallelImagePulls`.
- **FR-007**: System MUST use `resource.Quantity` for `systemReserved` and `kubeReserved`
  map values, and `metav1.Duration` for `evictionSoftGracePeriod` values.
- **FR-008**: System MUST enforce enum validation for `topologyManagerPolicy` (`none`,
  `best-effort`, `restricted`, `single-numa-node`), `cpuManagerPolicy` (`none`, `static`),
  and `memoryManagerPolicy` (`None`, `Static`).
- **FR-009**: System MUST enforce CEL cross-field validations:
  `imageGCHighThresholdPercent > imageGCLowThresholdPercent`,
  `evictionSoftGracePeriod` keys subset of `evictionSoft` keys.
- **FR-010**: System MUST enforce map key validation via CEL: `systemReserved`/`kubeReserved`
  keys in `{cpu, memory, ephemeral-storage, pid}`, eviction signal keys in
  `{memory.available, nodefs.available, nodefs.inodesFree, imagefs.available,
  imagefs.inodesFree, pid.available}`.
- **FR-011**: System MUST enforce via webhook: `cpuManagerPolicy=static` requires CPU
  reservation in `systemReserved` or `kubeReserved`; conflicting deprecated
  `maxParallelImagePullsPerNode` emits a warning.
- **FR-012**: System MUST set `maxProperties` on all map fields to keep CEL cost budgets
  bounded (4 for resource maps, 6 for eviction signal maps).
- **FR-013**: Eviction value format validation (`evictionHard`/`evictionSoft` string
  values) MUST be performed in the webhook, not CEL, to avoid cost budget risk.
- **FR-014**: System MUST register the mutation handler for all kubeadm-based providers
  (AWS, Docker, Nutanix) but NOT EKS.

### Key Entities

- **KubeletConfiguration**: New API type in `api/v1alpha1/` with 17 fields, 3 enum
  types, and mixed map value types.
- **KubeletConfiguration mutation handler**: New handler in
  `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/` that reads the variable,
  merges cluster/node-level config, and produces a strategic merge patch file.
- **KubeletConfiguration webhook validator**: New validator method added to the existing
  `MultiValidatingHandler` chain for cross-field and format validations too complex for CEL.

## Success Criteria

### Measurable Outcomes

- **SC-001**: All 17 KubeletConfiguration fields are configurable via cluster variables
  and correctly rendered into the strategic merge patch file.
- **SC-002**: Node-level overrides correctly take precedence over cluster-level defaults
  in all test scenarios.
- **SC-003**: Invalid input is rejected at apply-time with clear, actionable error messages
  for all validation rules (CEL + webhook).
- **SC-004**: Existing clusters using `maxParallelImagePullsPerNode` continue to work
  without modification.
- **SC-005**: Unit tests cover all 17 fields, merge semantics, deprecation path, and
  all validation rules. Integration tests cover the mutation handler end-to-end.
