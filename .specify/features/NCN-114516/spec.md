<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Opt-in Multus Meta-CNI Addon under CNI Configuration

**Jira Ticket**: [NCN-114516](https://jira.nutanix.com/browse/NCN-114516)
**Feature Branch**: `NCN-114516-opt-in-multus`
**Created**: 2026-11-25
**Status**: Draft
**Input**: Make Multus opt-in under `clusterConfig.addons.cni.multus` across all providers (`GenericCNI`), support custom Helm values via `AddonConfig` (`values.sourceRef`) with automatic `daemonConfig.readinessIndicatorFile` population, preserve existing Multus installations on upgrade when `HelmChartProxy` already exists, pre-populate `multus: {}` in CNI example patches, wire missing Multus flags, and clean up dead fields in the default Multus values template.

## Background

Today `MultusHandler` (`pkg/handlers/lifecycle/cni/multus/`) automatically deploys the `multus` Helm chart on every Nutanix and EKS cluster whenever `clusterConfig.addons.cni` is set, with no representation in the ClusterClass variable schema:

- Operators generating a Cluster manifest (e.g. dry-run or quick-start template) cannot see that Multus will be deployed and cannot opt out by removing it from the manifest.
- Operators cannot supply per-cluster Helm value overrides (such as `nodeSelector`, `tolerations`, `resources`, or `daemonConfig.logLevel`) without manual workload-cluster edits that are overwritten on upgrade.
- Docker and AWS clusters cannot opt in to Multus, preventing fast `provider:Docker` e2e coverage.
- `MultusConfig.AddFlags` is never wired into `lifecycle.Handlers.AddFlags` or the chart `Deployment`, and `charts/cluster-api-runtime-extensions-nutanix/addons/cni/multus/values-template.yaml` contains unused `volumes` and `volumeMounts` blocks that the `multus:0.1.1` chart ignores.

## User Scenarios & Testing

### User Story 1 - Opt in or opt out of Multus via `clusterConfig.addons.cni.multus` (Priority: P1)

An operator provisions a Cluster on any supported CAREN infrastructure provider (AWS, Docker, EKS, or Nutanix). When `clusterConfig.addons.cni.multus` is specified, CAREN deploys Multus via `HelmAddon` configured with the readiness socket path for the selected primary CNI (`Cilium`, `Calico`, or `Flow`). When `clusterConfig.addons.cni.multus` is omitted (including when removed from a generated quick-start manifest before cluster creation), CAREN does not deploy Multus.

**Why this priority**: Core requirement of [NCN-114516](https://jira.nutanix.com/browse/NCN-114516) — exposing Multus in the ClusterClass variable schema so customers can explicitly opt in or opt out.

**Independent Test**: Validate `ClusterConfig` variable discovery across AWS, Docker, EKS, and Nutanix schemas with and without `addons.cni.multus`, and verify `MultusHandler` creates `HelmChartProxy` `multus-<cluster-uuid>` on new clusters if and only if `addons.cni.multus` is present.

**Acceptance Scenarios**:

1. **Given** a new Cluster on AWS, Docker, EKS, or Nutanix with `clusterConfig.addons.cni` set and `cni.multus: {}` configured, **When** `AfterControlPlaneInitialized` runs, **Then** `MultusHandler` deploys `HelmChartProxy` `multus-<cluster-uuid>` using the `HelmAddon` strategy (defaulted by schema) and the primary CNI's readiness socket path.
2. **Given** a new Cluster on any provider (including Nutanix and EKS) with `clusterConfig.addons.cni` set and `cni.multus` omitted, **When** `AfterControlPlaneInitialized` runs, **Then** `MultusHandler` skips deployment and no `multus-<cluster-uuid>` `HelmChartProxy` is created.
3. **Given** the generated quick-start Cluster manifests in `examples/capi-quick-start/`, **When** inspecting `clusterConfig.addons.cni`, **Then** `multus: {}` is pre-populated alongside the primary CNI so operators see it in dry-run output and can delete `multus: {}` to opt out before applying.

---

### User Story 2 - Customize Multus Helm values via `values.sourceRef` ConfigMap (Priority: P1)

An operator wants to restrict Multus to a specific worker pool (`nodeSelector` / `tolerations`), adjust CPU limits (`resources`), or reduce log verbosity (`daemonConfig.logLevel: error`) without breaking primary-CNI socket readiness ordering or losing changes on cluster upgrades. They reference a `ConfigMap` in the Cluster's namespace via `clusterConfig.addons.cni.multus.values.sourceRef`.

**Why this priority**: Directly addresses day-2 node-targeting and upgrade persistence in [NCN-114516](https://jira.nutanix.com/browse/NCN-114516) without introducing Multus-specific API fields.

**Independent Test**: Configure `cni.multus.values.sourceRef` pointing to a `ConfigMap` containing `nodeSelector` and `daemonConfig.logLevel` (without `readinessIndicatorFile`). Verify `MultusHandler` sets the Cluster `OwnerReference` on the `ConfigMap`, preserves user values, and injects `daemonConfig.readinessIndicatorFile` for the primary CNI.

**Acceptance Scenarios**:

1. **Given** `cni.multus.values.sourceRef` references a `ConfigMap` in `cluster.Namespace` whose `values.yaml` omits `daemonConfig.readinessIndicatorFile`, **When** `MultusHandler` renders values, **Then** the resulting `HelmChartProxy.Spec.ValuesTemplate` contains all user-provided fields plus `daemonConfig.readinessIndicatorFile` set to the primary CNI's readiness socket path, and the `ConfigMap` has a Cluster `OwnerReference`.
2. **Given** `cni.multus.values.sourceRef` references a `ConfigMap` whose `values.yaml` explicitly sets a non-empty `daemonConfig.readinessIndicatorFile`, **When** `MultusHandler` renders values, **Then** the user's explicit `readinessIndicatorFile` value is preserved.
3. **Given** `cni.multus.values.sourceRef` is not set, **When** `MultusHandler` renders values, **Then** it uses the default ConfigMap (`default-multus-values-template`, overridable via `--cni.multus.helm-addon.default-values-template-configmap-name`) in CAREN's defaults namespace.

---

### User Story 3 - Non-disruptive CAREN upgrade for existing Clusters (Priority: P1)

An operator upgrades CAREN on a management cluster managing existing Nutanix and EKS workload clusters that had Multus auto-deployed by an earlier CAREN release (so `multus-<cluster-uuid>` `HelmChartProxy` exists, while `clusterConfig.addons.cni.multus` is unset in `Cluster` spec). Upgrading CAREN neither triggers Machine rollouts nor strands existing Multus installations on subsequent Kubernetes version upgrades, and does not mutate `Cluster` specs in etcd (avoiding GitOps `OutOfSync` drift).

**Why this priority**: Constitution Principle VII (no silent rollouts and safe CAREN upgrades).

**Independent Test**: Run `MultusHandler.BeforeClusterUpgrade` against a Cluster with `cni.multus == nil` and an existing `multus-<cluster-uuid>` `HelmChartProxy`; verify the `HelmChartProxy` is reconciled to the current chart and values without mutating the `Cluster` object. Also verify topology mutation `GeneratePatches` output is unchanged.

**Acceptance Scenarios**:

1. **Given** an existing Cluster where `cni.multus` is unset and `HelmChartProxy` `multus-<cluster-uuid>` already exists in `cluster.Namespace`, **When** `BeforeClusterUpgrade` (or `AfterControlPlaneInitialized`) runs, **Then** `MultusHandler` logs that it is reconciling an existing Multus installation and updates `multus-<cluster-uuid>` using the default values template without modifying `Cluster.spec`.
2. **Given** an existing Cluster where `cni.multus` is unset and an operator deletes `HelmChartProxy` `multus-<cluster-uuid>` to uninstall Multus day-2, **When** `BeforeClusterUpgrade` subsequently runs, **Then** `MultusHandler` sees no `HelmChartProxy` and does not re-install Multus.
3. **Given** any existing Cluster across all providers, **When** CAREN is upgraded, **Then** no topology mutation handler version bump is needed and `GeneratePatches` output is byte-identical.

### Edge Cases

- **`cni` omitted completely**: Because `multus` is nested under `addons.cni`, `multus` cannot be set in `Cluster` spec without `addons.cni`. If `addons.cni` is absent at runtime, `MultusHandler` skips cleanly.
- **Invalid or empty `cni.multus.strategy`**: OpenAPI schema restricts `strategy` to `Enum=HelmAddon` with `+kubebuilder:default=HelmAddon`. If an empty or unsupported strategy reaches `MultusHandler.apply()` directly, the handler fails the hook (`ResponseStatusFailure`) with a descriptive message.
- **Missing custom values `ConfigMap`**: If `cni.multus.values.sourceRef` points to a `ConfigMap` that does not exist in `cluster.Namespace`, `MultusHandler` fails the hook (`ResponseStatusFailure`) and surfaces the lookup error so CAPI retries until the `ConfigMap` is created.
- **Custom values `ConfigMap` with empty `values.yaml` or comment-only YAML**: Parsing treats empty YAML as an empty map and injects `daemonConfig.readinessIndicatorFile` cleanly without panicking on nil maps.
- **Primary CNI using `ClusterResourceSet` strategy**: `cni.strategy: ClusterResourceSet` with `cni.multus: {}` (`strategy: HelmAddon`) remains valid; `MultusHandler` derives the readiness socket path from `cni.provider` regardless of how the primary CNI was deployed.

## Requirements

### Functional Requirements

- **FR-001**: `api/v1alpha1` MUST extract shared CNI fields (`Strategy`, `AddonConfig`, `ImagePullCredentials`, and `Multus *Multus`) into `GenericCNI` and inline `GenericCNI` into both `CNI` and `NutanixCNI` so `addons.cni.multus` is available on AWS, Docker, EKS, and Nutanix.
- **FR-002**: `api/v1alpha1` MUST define `Multus` with `Strategy AddonStrategy` (`+kubebuilder:default=HelmAddon`, `+kubebuilder:validation:Optional`, `+kubebuilder:validation:Enum=HelmAddon`, `json:"strategy,omitzero"`) and inlined `AddonConfig` (`values.sourceRef`), with no Multus-specific typed fields.
- **FR-003**: `MultusHandler` MUST support all CAREN infrastructure providers (removing the `eks` / `nutanix`-only provider restriction).
- **FR-004**: When `addons.cni.multus` is set, `MultusHandler` MUST validate `Strategy == AddonStrategyHelmAddon` and deploy/reconcile `HelmChartProxy` `multus-<cluster-uuid>` in `cluster.Namespace`.
- **FR-005**: When `addons.cni.multus` is nil, `MultusHandler` MUST check whether `HelmChartProxy` `multus-<cluster-uuid>` already exists in `cluster.Namespace`; if absent it MUST skip without error, and if present it MUST reconcile the existing `HelmChartProxy` using the default values template.
- **FR-006**: When `addons.cni.multus.values.sourceRef` is set, `MultusHandler` MUST read `values.yaml` from that `ConfigMap` in `cluster.Namespace`, ensure a Cluster `OwnerReference` on the `ConfigMap`, execute Go template expansion with `.ReadinessSocketPath`, and inject `daemonConfig.readinessIndicatorFile` set to the primary CNI's readiness socket path whenever `daemonConfig.readinessIndicatorFile` is unset or empty in the rendered YAML.
- **FR-007**: `charts/cluster-api-runtime-extensions-nutanix/addons/cni/multus/values-template.yaml` MUST remove the unused `volumes` and `volumeMounts` blocks that `multus:0.1.1` ignores.
- **FR-008**: `lifecycle.Handlers.AddFlags` MUST register `h.multusConfig.AddFlags("cni.multus", flagSet)` and `charts/cluster-api-runtime-extensions-nutanix/templates/deployment.yaml` MUST pass `--cni.multus.helm-addon.default-values-template-configmap-name`.
- **FR-009**: `hack/examples/patches/cilium.yaml`, `hack/examples/patches/calico.yaml`, `hack/examples/patches/nutanix-flow.yaml`, and `hack/examples/patches/eks/initialize-variables.yaml` MUST pre-populate `multus: {}` under `cni`, with `examples/` regenerated via `make examples.sync`.
- **FR-010**: E2E helper `WaitForCNIToBeReadyInWorkloadCluster` (`test/e2e/cni_helpers.go`) MUST wait for the `multus` `HelmReleaseProxy` and `kube-system/multus` `DaemonSet` when `CNI.Multus != nil`.
- **FR-011**: User documentation in `docs/content/addons/cni.md` MUST document `addons.cni.multus` (opting in/out, custom `values.sourceRef` ConfigMap behavior, day-2 removal, and Cilium `cni.exclusive: false` requirement when using custom Cilium values).
- **FR-012**: Upgrading CAREN MUST NOT alter any topology mutation handler's `GeneratePatches` output or bump mutation handler versions.

### Key Entities

- **`GenericCNI`** (`api/v1alpha1/addon_types.go`): Shared CNI addon fields inlined into `CNI` and `NutanixCNI`, holding `Strategy`, `AddonConfig`, `ImagePullCredentials`, and `Multus`.
- **`Multus`** (`api/v1alpha1/addon_types.go`): Opt-in Multus addon configuration under `cni.multus`, holding `Strategy` (`HelmAddon`) and `AddonConfig` (`values.sourceRef`).
- **`MultusHandler`** (`pkg/handlers/lifecycle/cni/multus/`): Lifecycle hook handler (`AfterControlPlaneInitialized`, `BeforeClusterUpgrade`) managing the `multus-<cluster-uuid>` `HelmChartProxy`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: `DiscoverVariables` validation tests pass across AWS, Docker, EKS, and Nutanix schemas for default `multus: {}`, explicit `strategy: HelmAddon`, custom `values.sourceRef`, and rejected invalid strategies.
- **SC-002**: Unit tests for `MultusHandler` and `template.go` cover opt-in deployment, opt-out skip on new clusters, upgrade reconciliation when `multus-<cluster-uuid>` `HelmChartProxy` already exists, custom `values.sourceRef` with automatic `readinessIndicatorFile` injection and explicit override preservation, and error handling.
- **SC-003**: `make go-generate`, `make examples.sync`, `make chart-docs`, `make schema-chart`, `make test`, `make lint`, and `make pre-commit` all pass with a clean git tree aside from intended source and regenerated files.
- **SC-004**: `provider:Docker` quick-start e2e deploys Multus alongside Cilium and passes `WaitForCNIToBeReadyInWorkloadCluster` readiness checks.
