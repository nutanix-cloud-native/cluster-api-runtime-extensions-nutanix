<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Feature Specification: Expose `imageMaximumGCAge` and `imageMinimumGCAge` in KubeletConfiguration

**Jira Ticket**: [NCN-119707](https://jira.nutanix.com/browse/NCN-119707)
**Feature Branch**: `NCN-119707-kubelet-image-gc-age`
**Created**: 2026-11-25
**Status**: Draft
**Input**: [NCN-119707](https://jira.nutanix.com/browse/NCN-119707) — expose persistent age-based image garbage collection (`imageMaximumGCAge` and `imageMinimumGCAge`) in CAREN `KubeletConfiguration`.

## User Scenarios & Testing

### User Story 1 - Configure age-based cleanup of unused images (Priority: P1)

A cluster administrator configures `imageMaximumGCAge` (for example, `"48h"`) on worker pools (`workerConfig.kubeletConfiguration`) or control plane nodes (`clusterConfig.controlPlane.kubeletConfiguration`) so the kubelet automatically garbage-collects container images that have remained unused for longer than that duration, regardless of disk usage, without manual edits to `/var/lib/kubelet/config.yaml`.

**Why this priority**: Solves the core customer requirement in [NCN-119707](https://jira.nutanix.com/browse/NCN-119707) in a lifecycle-safe, declarative way across node replacements, scale-outs, reboots, and upgrades.

**Independent Test**: Render `GeneratePatches` for a `Cluster` with `workerConfig.kubeletConfiguration.imageMaximumGCAge: "48h"` and verify `/etc/kubernetes/patches/kubeletconfiguration99+strategic.json` unmarshals into an upstream `kubelet.config.k8s.io/v1beta1.KubeletConfiguration` with `ImageMaximumGCAge == 48h`.

**Acceptance Scenarios**:

1. **Given** a `Cluster` with `workerConfig.kubeletConfiguration.imageMaximumGCAge: "48h"`, **When** topology mutation runs, **Then** the worker `KubeadmConfigTemplate` contains `/etc/kubernetes/patches/kubeletconfiguration99+strategic.json` with `imageMaximumGCAge: 48h0m0s`.
2. **Given** a `Cluster` with `clusterConfig.controlPlane.kubeletConfiguration.imageMaximumGCAge: "48h"`, **When** topology mutation runs, **Then** the `KubeadmControlPlaneTemplate` contains `/etc/kubernetes/patches/kubeletconfiguration99+strategic.json` with `imageMaximumGCAge: 48h0m0s`.
3. **Given** an existing `Cluster` where neither `imageMaximumGCAge` nor `imageMinimumGCAge` is set, **When** CAREN is upgraded, **Then** `GeneratePatches` output is byte-for-byte identical and no Machine rollout occurs.

---

### User Story 2 - Tune minimum image age alongside maximum image age (Priority: P2)

A cluster administrator configures `imageMinimumGCAge` (for example, `"10m"` or `"1h"`) on a node group to control the minimum age since an image was first seen before disk-pressure GC can evict it, optionally combining it with `imageMaximumGCAge` and `imageGCHighThresholdPercent` / `imageGCLowThresholdPercent`.

**Why this priority**: Completes the four upstream kubelet image-GC settings (`imageMinimumGCAge`, `imageMaximumGCAge`, `imageGCHighThresholdPercent`, `imageGCLowThresholdPercent`) and allows adjusting minimum-age protection when setting `imageMaximumGCAge`.

**Independent Test**: Render `GeneratePatches` with `imageMinimumGCAge: "15m"` and `imageMaximumGCAge: "48h"`, and verify both fields deserialize accurately in the rendered `KubeletConfiguration` patch.

**Acceptance Scenarios**:

1. **Given** `kubeletConfiguration` with `imageMinimumGCAge: "15m"`, **When** topology mutation runs, **Then** the rendered strategic merge patch contains `imageMinimumGCAge: 15m0s`.
2. **Given** `kubeletConfiguration` with both `imageMinimumGCAge: "15m"` and `imageMaximumGCAge: "48h"`, **When** topology mutation runs, **Then** both fields are rendered into the strategic merge patch.

---

### User Story 3 - Reject kubelet-breaking image GC durations at admission (Priority: P1)

A cluster administrator submits an invalid `imageMaximumGCAge` or `imageMinimumGCAge` value (such as a negative duration, `imageMinimumGCAge: "0s"`, or `imageMaximumGCAge: "1m"` which is not strictly greater than kubelet's `"2m"` default `imageMinimumGCAge`). Admission rejects the `Cluster` immediately with a clear field-scoped error instead of rolling out Machines whose kubelet fails startup validation.

**Why this priority**: Upstream `ValidateKubeletConfiguration` crashes the kubelet at startup when `imageMaximumGCAge > 0 && imageMaximumGCAge <= imageMinimumGCAge` (default `2m`) or when either duration is negative. Failing fast at `Cluster` admission prevents broken node rollouts (the same class of defect seen in [NCN-116355](https://jira.nutanix.com/browse/NCN-116355)).

**Independent Test**: Submit `Cluster` admission requests and `DiscoverVariables` schema validation cases covering negative durations, zero `imageMinimumGCAge`, `imageMaximumGCAge <= 2m` (when `imageMinimumGCAge` is unset), and `imageMaximumGCAge <= imageMinimumGCAge` (when both are set); verify rejection with actionable messages.

**Acceptance Scenarios**:

1. **Given** `imageMaximumGCAge: "1m"` and `imageMinimumGCAge` unset, **When** validating the `Cluster`, **Then** validation rejects the request because `imageMaximumGCAge` (`1m0s`) must be greater than the default `imageMinimumGCAge` (`2m0s`).
2. **Given** `imageMinimumGCAge: "30m"` and `imageMaximumGCAge: "15m"`, **When** validating the `Cluster`, **Then** validation rejects the request because `imageMaximumGCAge` must be greater than `imageMinimumGCAge`.
3. **Given** `imageMinimumGCAge: "30s"` and `imageMaximumGCAge: "1m"`, **When** validating the `Cluster`, **Then** validation accepts the request because `1m > 30s`.
4. **Given** `imageMaximumGCAge: "0s"` (explicitly disabled), **When** validating the `Cluster`, **Then** validation accepts the request regardless of `imageMinimumGCAge`.
5. **Given** `imageMinimumGCAge: "0s"` or a negative duration on either field, **When** validating the `Cluster`, **Then** validation rejects the request.

### Edge Cases

- **`imageMaximumGCAge: "0s"` vs unset**: Upstream kubelet treats `"0s"` as disabled. If a user explicitly sets `"0s"`, validation allows it (`> 0` check does not fire) and the patch renders `imageMaximumGCAge: 0s`; if unset (`nil`), nothing is emitted in the patch so existing clusters see no diff.
- **`imageMinimumGCAge` unset when `imageMaximumGCAge` is small**: Upstream kubelet defaults `imageMinimumGCAge` to `2m` before running `ValidateKubeletConfiguration`. Validation MUST compare `imageMaximumGCAge` against `2m` when `imageMinimumGCAge` is `nil`.
- **`imageMinimumGCAge` lowered below `2m`**: If the user sets `imageMinimumGCAge: "30s"` and `imageMaximumGCAge: "1m"`, upstream kubelet accepts it (`1m > 30s`); CAREN validation MUST allow this rather than hardcoding a `2m` floor when `imageMinimumGCAge` is explicitly provided.
- **Unparseable or non-Go duration strings**: `metav1.Duration` unmarshaling and CRD schema/CEL `duration()` validation reject malformed strings (for example `"2d"` instead of `"48h"`). Documentation MUST call out that Go duration units up to hours (`h`, `m`, `s`) are used (`"48h"`, not `"2d"`).
- **Worker pool overrides**: Validation MUST enforce the rules on `clusterConfig.controlPlane.kubeletConfiguration`, global `workerConfig.kubeletConfiguration`, and per-`MachineDeployment` `workerConfig.kubeletConfiguration` overrides, reporting the exact field path on error.
- **EKS clusters**: Excluded, consistent with all other `kubeadm` `KubeletConfiguration` fields.

## Requirements

### Functional Requirements

- **FR-001**: `v1alpha1.KubeletConfiguration` MUST add an optional `ImageMinimumGCAge *metav1.Duration` field (`json:"imageMinimumGCAge,omitempty"`).
- **FR-002**: `v1alpha1.KubeletConfiguration` MUST add an optional `ImageMaximumGCAge *metav1.Duration` field (`json:"imageMaximumGCAge,omitempty"`).
- **FR-003**: CRD / OpenAPI CEL validation (`+kubebuilder:validation:XValidation`) on `KubeletConfiguration` and the `Cluster` admission validator (`kubeletConfigurationValidator`) MUST enforce:
  - `imageMinimumGCAge`, when set, is strictly greater than `0s` (`duration(self) > duration('0s')`).
  - `imageMaximumGCAge`, when set, is greater than or equal to `0s` (`duration(self) >= duration('0s')`).
  - When `imageMaximumGCAge` is set and greater than `0s`:
    - If `imageMinimumGCAge` is set, `imageMaximumGCAge > imageMinimumGCAge`.
    - If `imageMinimumGCAge` is unset, `imageMaximumGCAge > 2m` (upstream kubelet's default `imageMinimumGCAge`).
- **FR-004**: `kubeletconfiguration` template rendering (`toTemplateInput` and `embedded/kubeletconfigpatch.yaml.tmpl`) MUST emit `imageMinimumGCAge` and `imageMaximumGCAge` (formatted via `Duration.String()`) only when the respective pointer is non-`nil`.
- **FR-005**: When both `ImageMinimumGCAge` and `ImageMaximumGCAge` are `nil`, `GeneratePatches` output for all existing Cluster configurations MUST remain byte-for-byte identical. No mutation handler version bump is permitted or required.
- **FR-006**: `make go-generate` MUST update deepcopy methods and all provider ClusterConfig / WorkerNodeConfig CRD schemas cleanly.
- **FR-007**: `docs/content/customization/kubeadm/kubelet-configuration.md` MUST be updated to document `imageMinimumGCAge` and `imageMaximumGCAge`, including:
  - Supported units (`"48h"`, not `"2d"`), defaults (`"2m"` and `"0s"` disabled), and the `imageMaximumGCAge > imageMinimumGCAge` constraint.
  - How disk-threshold GC (`imageGCHighThresholdPercent` / `imageGCLowThresholdPercent`) and age-based GC (`imageMaximumGCAge`) operate independently and only ever remove unused images (never images referenced by running/active containers).
  - Rollout behavior (changing `kubeletConfiguration` updates `KubeadmConfigTemplate` / `KubeadmControlPlaneTemplate` and triggers a rolling node replacement) and how operators verify rollout state (`Cluster` / `MachineDeployment` / `Machine` conditions) and effective node configuration (`/api/v1/nodes/<node>/proxy/configz`).

### Key Entities

- **`v1alpha1.KubeletConfiguration`**: Strongly-typed kubelet configuration struct shared by `KubeadmNodeSpec` for both control plane (`clusterConfig.controlPlane.kubeletConfiguration`) and worker pools (`workerConfig.kubeletConfiguration`).
- **Kubelet strategic merge patch (`/etc/kubernetes/patches/kubeletconfiguration99+strategic.json`)**: File injected into `KubeadmConfigSpec.Files` and applied by `kubeadm init` / `kubeadm join` to produce `/var/lib/kubelet/config.yaml`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Unit and variable-discovery tests verify schema/CEL acceptance and rejection for all valid and invalid combinations of `imageMinimumGCAge` and `imageMaximumGCAge` across Docker, AWS, and Nutanix `ClusterConfig` / `WorkerNodeConfig` variables.
- **SC-002**: Admission webhook tests verify `kubeletConfigurationValidator` rejects invalid `imageMinimumGCAge` / `imageMaximumGCAge` values on control plane, default worker config, and per-`MachineDeployment` worker overrides with accurate field paths.
- **SC-003**: Template rendering tests deserialize the rendered YAML into `k8s.io/kubelet/config/v1beta1.KubeletConfiguration` and verify exact `metav1.Duration` round-tripping.
- **SC-004**: Existing `GeneratePatches` tests pass unchanged, confirming zero impact on clusters not setting the new fields.
- **SC-005**: `make lint` and `make pre-commit` (including markdownlint on updated docs) pass with zero errors.

## Out of Scope

- **Kubernetes version-gated admission checks**: Upstream `ImageMaximumGCAge` (KEP-4210) has been Beta and enabled by default since Kubernetes v1.30 and GA since v1.35; all NKP releases supporting CAREN `kubeletConfiguration` (NKP 2.18+, Kubernetes v1.35+) support it natively without feature gates.
- **Workload-cluster `/configz` polling or per-worker drift status controllers**: CAREN is a stateless Runtime Extension; rollout status is reported by Cluster API resources and live per-node configuration is exposed via Kubernetes' native `/api/v1/nodes/<node>/proxy/configz` endpoint and `kubelet_image_manager_*` metrics.
- **EKS clusters**: EKS nodes do not use kubeadm strategic merge patches.
