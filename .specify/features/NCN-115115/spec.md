<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Opt-in automatic kubelet resource reservations sized to the node

**Jira Ticket**: [NCN-115115](https://jira.nutanix.com/browse/NCN-115115)
**Feature Branch**: `NCN-115115-auto-kubelet-reservations`
**Created**: 2026-06-11
**Status**: Draft
**Input**: User description: "now we have kubelet configuration available for system reserved
resources, i would like to make this easier for customers by some default automated settings
(opt-in) depending on node size, workload, etc. explicit values in kubelet configuration and
automatic configuration should be mutually exclusive. investigate how GKE and EKS tackle this
problem and brainstorm an approach. as always, everything should be opt-in in CAREN so defaults
remain as they currently are and a no-op when we introduce this configuration"

## Background

CAREN already exposes raw kubelet `systemReserved`/`kubeReserved` knobs on both
`clusterConfig.controlPlane.kubeletConfiguration` and
`workerConfig.kubeletConfiguration`. These require customers to hand-pick concrete
CPU/memory values per node type. The correct value is node-size dependent, so a single
static setting cannot serve a fleet of differently-sized machine deployments. Picking
too low destabilises nodes (daemons starve, `NotReady`, OOM kills); picking too high
wastes allocatable capacity and reduces pod density.

Managed Kubernetes providers solve this by computing reservations **on the node at
boot, scaled to the node's actual capacity**:

- **GKE** uses tiered percentages of total capacity. Memory: 255Mi if total <1Gi; else
  25% of the first 4Gi + 20% of the next 4Gi + 10% of the next 8Gi + 6% of the next
  112Gi + 2% above 128Gi, plus a 100Mi hard-eviction threshold. CPU: 6% of the first
  core + 1% of the 2nd + 0.5% of cores 3-4 + 0.25% of cores beyond 4. The entire
  reservation is placed in `kubeReserved`; `system-reserved` is not separately set.
- **EKS** computes in its node bootstrap (`bootstrap.sh` / `nodeadm`): `kubeReserved`
  memory ≈ `255Mi + 11Mi × maxPods`, CPU uses the same GKE-style tiered formula, both
  written to `kubeReserved`.

The common thread: the reservation depends on the node's real CPU/memory, so the
computation runs where that is known — on the node.

## User Scenarios & Testing

### User Story 1 - Operator enables automatic reservations with one switch (Priority: P1)

An operator running a fleet of differently-sized worker machine deployments wants node
stability comparable to GKE/EKS without hand-calculating `kubeReserved` for every
instance size. They set a single opt-in field on their `kubeletConfiguration` and each
node reserves resources scaled to its own capacity.

**Why this priority**: This is the core value of the feature — turnkey, size-aware
reservations that remove expert-only tuning.

**Independent Test**: On a workload cluster with two worker machine deployments of
different sizes (e.g. 2 vCPU / 8Gi and 16 vCPU / 64Gi), enable automatic reservations.
Verify each node's `kubeReserved` (visible via `kubectl get node -o jsonpath` on
`status.allocatable` vs `status.capacity`, or on-node kubelet config) reflects a
smaller proportional reservation on the larger node, following the tiered formula.

**Acceptance Scenarios**:

1. **Given** a worker `kubeletConfiguration` with automatic reservations enabled and no
   explicit `systemReserved`/`kubeReserved`/`evictionHard`, **When** a worker node of
   size 2 vCPU / 8Gi boots, **Then** its kubelet runs with `kubeReserved`
   `cpu=70m, memory=1843Mi` and `evictionHard memory.available=100Mi`.
2. **Given** the same configuration, **When** a worker node of size 16 vCPU / 64Gi
   boots, **Then** its kubelet runs with `kubeReserved` `cpu=110m` and a memory value
   computed by the tiered formula, and `evictionHard memory.available=100Mi`.
3. **Given** automatic reservations enabled on `clusterConfig.controlPlane.kubeletConfiguration`,
   **When** a control plane node boots, **Then** its kubelet runs with the
   formula-computed `kubeReserved` and `evictionHard` for the control plane node's size.

### User Story 2 - Operator upgrades CAREN on working clusters (Priority: P1)

An operator upgrades CAREN on a management cluster with many running workload clusters
that do not use this new field. They expect zero churn.

**Why this priority**: Constitution principle on no silent rollouts. The feature must be
a no-op when unset and must not bump any topology mutation handler version.

**Independent Test**: Compare `GeneratePatches` output of the kubelet configuration
handler before and after the change for a representative Cluster that does not set the
new field. Output must be byte-identical.

**Acceptance Scenarios**:

1. **Given** an existing managed Cluster that does not set automatic reservations,
   **When** CAREN is upgraded to the version including this change, **Then** no
   MachineDeployment or KubeadmControlPlane is rolled out as a result of the upgrade.

### User Story 3 - Operator is prevented from mixing automatic and explicit values (Priority: P2)

An operator mistakenly sets both automatic reservations and an explicit `kubeReserved`.
They get a clear admission error rather than silently-conflicting configuration.

**Why this priority**: Prevents an ambiguous/confusing blend of the two models. Lower
than P1 because it is a guardrail, not the primary capability.

**Independent Test**: Apply a Cluster whose `kubeletConfiguration` sets both automatic
reservations and explicit `systemReserved` (or `kubeReserved`, or `evictionHard`).
Verify the request is rejected at admission with a message naming the conflicting fields.

**Acceptance Scenarios**:

1. **Given** a `kubeletConfiguration` with automatic reservations enabled, **When** the
   same `kubeletConfiguration` also sets `systemReserved`, `kubeReserved`, or
   `evictionHard`, **Then** the create/update is rejected at admission with a clear
   error.
2. **Given** a `kubeletConfiguration` with automatic reservations enabled and other
   non-reservation kubelet fields set (e.g. `maxPods`, `containerLogMaxSize`),
   **When** the Cluster is applied, **Then** it is accepted and both the automatic
   reservation behaviour and the other kubelet fields take effect.

### Edge Cases

- **Very small node (<1Gi memory):** the formula reserves a flat 255Mi memory; CPU
  reservation is 6% of the first core. Must not produce a reservation that exceeds node
  capacity.
- **Single-core node:** CPU reservation is 60m (6% of one core), with no higher tiers
  applied.
- **Very large node (e.g. 256 vCPU / 1Ti):** higher tiers apply with their small
  percentages; values must be finite and correctly summed across tiers.
- **Capacity detection fails on the node:** the boot script must fail loudly (non-zero
  exit) so bootstrap surfaces the error rather than silently producing no reservation.
- **The CIS-hardening default ClusterClass already writes an `evictionHard
  memory.available=100Mi` patch** at a lower-numbered patch file. The automatic
  reservation's `evictionHard memory.available=100Mi` is consistent with that value and
  must merge cleanly.
- **Automatic reservations is a CAREN directive, not a kubelet field:** it must never be
  rendered verbatim into the kubelet `KubeletConfiguration` patch.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST provide an opt-in field on `kubeletConfiguration`
  (available for both control plane and worker scopes) that enables automatic,
  node-size-based resource reservations, selecting a named profile. The initial profile
  MUST be `CapacityTiered`.
- **FR-002**: When automatic reservations are enabled, each node MUST compute its
  reservation at boot from its own actual CPU and memory capacity, before kubeadm runs,
  and apply it via the kubelet configuration patch mechanism already used by CAREN
  (`/etc/kubernetes/patches/`).
- **FR-003**: The `CapacityTiered` profile MUST compute reserved CPU as: 6% of the first
  core + 1% of the second core + 0.5% of cores three and four + 0.25% of every core
  beyond four, expressed in millicores.
- **FR-004**: The `CapacityTiered` profile MUST compute reserved memory as: 255Mi when
  total memory is below 1Gi; otherwise 25% of the first 4Gi + 20% of the next 4Gi + 10%
  of the next 8Gi + 6% of the next 112Gi + 2% of memory above 128Gi.
- **FR-005**: The computed CPU and memory reservations MUST be written to `kubeReserved`,
  and the profile MUST additionally set `evictionHard` `memory.available` to `100Mi`.
- **FR-006**: Automatic reservations MUST be mutually exclusive with explicit
  `systemReserved`, `kubeReserved`, and `evictionHard` on the same `kubeletConfiguration`.
  The combination MUST be rejected at admission (declaratively, via CRD validation) with
  a clear error.
- **FR-007**: Other non-reservation kubelet fields (e.g. `maxPods`,
  `containerLogMaxSize`) MUST continue to be honoured alongside automatic reservations.
- **FR-008**: The feature MUST be opt-in and a no-op when unset: for any input that does
  not set automatic reservations, the kubelet configuration handler's `GeneratePatches`
  output MUST be byte-identical to the pre-change output, and no handler version bump
  MUST be required.
- **FR-009**: The boot-time computation MUST be unit-testable deterministically by
  allowing the node CPU and memory inputs to be supplied/overridden, falling back to
  live capacity detection when not supplied.
- **FR-010**: The feature MUST be documented under
  `docs/content/customization/kubeadm/kubelet-configuration.md`, including what it does,
  how to enable it, the formula, the mutual-exclusivity constraint, and example YAML.

### Key Entities

- **Automatic reservations directive**: An opt-in block on `kubeletConfiguration`
  carrying a profile selector (initial value `CapacityTiered`). A CAREN-level instruction,
  not a kubelet field; it drives boot-time computation rather than being rendered into the
  kubelet config.
- **`CapacityTiered` profile**: The reservation strategy defining the CPU and memory
  tiered formulas and the fixed `evictionHard memory.available` value.
- **Boot-time reservation computation**: The on-node step that reads the node's CPU and
  memory capacity, evaluates the active profile, and writes a kubelet strategic-merge
  patch consumed by kubeadm.

## Success Criteria

### Measurable Outcomes

- **SC-001**: With automatic reservations enabled, on two worker nodes of materially
  different sizes, the larger node reserves a strictly smaller fraction of its total
  memory than the smaller node (demonstrating proportional scaling), and both match the
  `CapacityTiered` formula exactly for their size.
- **SC-002**: Enabling automatic reservations requires setting exactly one field
  (the profile selector) per `kubeletConfiguration` scope — no per-instance-type values.
- **SC-003**: Upgrading CAREN to the release including this change, on a management
  cluster with N workload clusters none of which set the new field, triggers zero
  MachineDeployment or KubeadmControlPlane rollouts attributable to this change.
- **SC-004**: Applying a `kubeletConfiguration` that sets both automatic reservations and
  any of `systemReserved`/`kubeReserved`/`evictionHard` is rejected at admission 100% of
  the time, with an error message naming the conflict.
