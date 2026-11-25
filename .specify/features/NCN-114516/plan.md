<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Opt-in Multus Meta-CNI Addon under CNI Configuration

**Branch**: `NCN-114516-opt-in-multus` | **Date**: 2026-11-25 | **Spec**: [./spec.md](./spec.md) | **Jira**: [NCN-114516](https://jira.nutanix.com/browse/NCN-114516)

## Summary

Expose Multus as an opt-in meta-CNI addon under `clusterConfig.addons.cni.multus` across all providers (AWS, Docker, EKS, Nutanix) by extracting `GenericCNI` in `api/v1alpha1/addon_types.go` and defining `Multus` with `Strategy` (`HelmAddon`) plus inlined `AddonConfig` (`values.sourceRef`). Update `MultusHandler` to deploy Multus on any provider when `addons.cni.multus` is set, reconcile existing `multus-<cluster-uuid>` `HelmChartProxy` resources on upgrade when `addons.cni.multus` is unset (without mutating `Cluster` specs), automatically populate `daemonConfig.readinessIndicatorFile` when custom `values.sourceRef` ConfigMaps omit it, wire missing `--cni.multus.helm-addon.default-values-template-configmap-name` flags, remove dead `volumes`/`volumeMounts` blocks from `addons/cni/multus/values-template.yaml`, pre-populate `multus: {}` in CNI example patches, add Multus e2e readiness checks, and document the feature in `docs/content/addons/cni.md`.

## Technical Context

**Language/Version**: Go (per `go.mod` / `api/go.mod`)
**Primary Dependencies**: `sigs.k8s.io/controller-runtime`, `sigs.k8s.io/cluster-api`, `sigs.k8s.io/yaml`, `github.com/stretchr/testify`
**Storage**: N/A (Kubernetes CRDs / `HelmChartProxy` / `ConfigMap`)
**Testing**: `testify` unit tests (`pkg/handlers/lifecycle/cni/`, `pkg/handlers/lifecycle/cni/multus/`, `pkg/handlers/lifecycle/addons/`), Ginkgo/Gomega e2e helpers (`test/e2e/cni_helpers.go`)
**Target Platform**: Kubernetes management cluster (CAREN runtime extension server)
**Project Type**: Multi-module Go repository (`api/`, `common/`, root) + Helm chart + Hugo docs
**Constraints**: No topology mutation `GeneratePatches` output changes or handler version bumps; no `Cluster` spec mutation on upgrade; pass `kube-api-linter`, `golangci-lint`, `yamllint`, and `markdownlint`
**Scale/Scope**: `api/v1alpha1/`, `pkg/handlers/lifecycle/`, `charts/`, `hack/examples/` + generated `examples/`, `test/e2e/`, `docs/content/addons/`

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| I. API-First | Pass | `GenericCNI` and `Multus` live in `api/v1alpha1/addon_types.go` with kubebuilder markers only; no business logic in `api/`. |
| II. Handler-per-Provider | Pass | Lifecycle handler stays provider-agnostic in `pkg/handlers/lifecycle/cni/multus/`; no cross-provider imports. |
| III. Library-First | Pass | All logic lives in `pkg/handlers/lifecycle/`; `cmd/main.go` unchanged. |
| IV. Tests Required | Pass | `testify` unit tests for variable discovery (`AWS`, `Docker`, `EKS`, `Nutanix`) and `MultusHandler` + template rendering; e2e helper updated. |
| V. Code Style | Pass | Mandatory CAPI import aliases (`clusterv1`, `runtimehooksv1`, etc.), `make fmt` and `make lint`. |
| VI. Dependency Management | Pass | Zero new dependencies (`sigs.k8s.io/yaml` is already a direct dependency). |
| VII. Handler Version Safety | Pass | Lifecycle handler only; `DiscoverVariables` schema addition does not alter `GeneratePatches` output. |
| VIII. Handler Documentation | Pass | `docs/content/addons/cni.md` updated in the same change. |

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-114516/
├── spec.md
└── plan.md
```

### Source Code (repository root)

```text
api/v1alpha1/
├── addon_types.go                      # MODIFY: extract GenericCNI, add Multus, inline GenericCNI into CNI and NutanixCNI
├── zz_generated.deepcopy.go            # GENERATED via make go-generate
└── crds/*.yaml                         # GENERATED via make go-generate

pkg/handlers/lifecycle/
├── handlers.go                         # MODIFY: wire h.multusConfig.AddFlags("cni.multus", flagSet)
├── addons/
│   └── helmaddon.go                    # MODIFY: add DefaultValuesTemplateConfigMapName() getter on HelmAddonConfig
└── cni/
    ├── variables_test.go               # MODIFY: add Multus variable validation cases + EKS coverage
    └── multus/
        ├── doc.go                      # MODIFY: update package doc for opt-in Multus addon
        ├── handler.go                  # MODIFY: all-provider opt-in, existing-HCP upgrade check, custom values.sourceRef + OwnerRef
        ├── template.go                 # MODIFY: template + ensureReadinessIndicatorFile YAML injection
        └── handler_test.go             # CREATE: testify unit tests for handler, flags, and template logic

charts/cluster-api-runtime-extensions-nutanix/
├── addons/cni/multus/
│   └── values-template.yaml            # MODIFY: remove unused volumes and volumeMounts blocks
├── templates/
│   └── deployment.yaml                 # MODIFY: pass --cni.multus.helm-addon.default-values-template-configmap-name
└── defaultclusterclasses/*.yaml        # GENERATED via make examples.sync (DiscoverVariables unchanged in ClusterClass definitions)

hack/examples/patches/
├── cilium.yaml                         # MODIFY: add multus: {}
├── calico.yaml                         # MODIFY: add multus: {}
├── nutanix-flow.yaml                   # MODIFY: add multus: {}
└── eks/
    └── initialize-variables.yaml       # MODIFY: add multus: {}

examples/                               # GENERATED via make examples.sync

test/e2e/
└── cni_helpers.go                      # MODIFY: wait for multus HelmReleaseProxy and DaemonSet when CNI.Multus != nil

docs/content/addons/
└── cni.md                              # MODIFY: add Multus configuration, custom values, and opt-out/day-2 docs
```

**Structure Decision**: Keep `MultusHandler` in `pkg/handlers/lifecycle/cni/multus/` alongside `calico`, `cilium`, and `nutanixflow`, matching its schema location (`addons.cni.multus`) and reusing `cni.ReadinessSocketPath`.

## Tasks

Atomic, TDD-ordered tasks executed inside `.worktrees/NCN-114516-opt-in-multus` via `devbox run --`.

### T1 — API Types (`GenericCNI`, `Multus`) and `DiscoverVariables` Tests

**Files**:

- Modify: `pkg/handlers/lifecycle/cni/variables_test.go`
- Modify: `api/v1alpha1/addon_types.go`
- Regenerate: `api/v1alpha1/zz_generated.deepcopy.go`, `api/v1alpha1/crds/*.yaml`

**Steps**:

- [ ] **Step 1 (Red)**: Extend `pkg/handlers/lifecycle/cni/variables_test.go` with `TestVariableValidation_EKS` and test cases setting `CNI.Multus` (`&v1alpha1.Multus{}`, explicit `Strategy: HelmAddon`, custom `Values.SourceRef`, invalid `Strategy: ClusterResourceSet` expecting error, plus Flow + Multus on Nutanix). Run `devbox run -- go test ./pkg/handlers/lifecycle/cni/` and confirm compilation fails on missing `Multus` field/type.
- [ ] **Step 2 (Green)**: In `api/v1alpha1/addon_types.go`, extract `GenericCNI` (`Strategy`, `AddonConfig`, `ImagePullCredentials`, `Multus *Multus`), inline `GenericCNI` into `CNI` and `NutanixCNI`, and define `Multus` (`Strategy` defaulted to `HelmAddon` with `Enum=HelmAddon`, plus inlined `AddonConfig`).
- [ ] **Step 3**: Run `devbox run -- make go-generate` so embedded CRD YAML schemas and deepcopy methods regenerate.
- [ ] **Step 4**: Re-run `devbox run -- go test ./pkg/handlers/lifecycle/cni/` and confirm all AWS, Docker, EKS, and Nutanix variable discovery tests pass.

**Completion Criteria**: `go test ./pkg/handlers/lifecycle/cni/` passes; `api/` builds cleanly; `FR-001`, `FR-002`, and `SC-001` satisfied.

---

### T2 — Default Values Template Cleanup and Flag Wiring

**Files**:

- Modify: `charts/cluster-api-runtime-extensions-nutanix/addons/cni/multus/values-template.yaml`
- Modify: `pkg/handlers/lifecycle/addons/helmaddon.go`
- Modify: `pkg/handlers/lifecycle/handlers.go`
- Modify: `charts/cluster-api-runtime-extensions-nutanix/templates/deployment.yaml`

**Steps**:

- [ ] **Step 1 (Red)**: Write a flag registration test in `pkg/handlers/lifecycle/cni/multus/handler_test.go` verifying `lifecycle.New(...).AddFlags(fs)` registers `cni.multus.helm-addon.default-values-template-configmap-name`, plus a template test asserting the rendered chart `values-template.yaml` contains only `daemonConfig` and `resources` (no `volumes` or `volumeMounts`). Confirm failure.
- [ ] **Step 2 (Green)**:
  - Add `DefaultValuesTemplateConfigMapName() string` on `addons.HelmAddonConfig` in `pkg/handlers/lifecycle/addons/helmaddon.go`.
  - Wire `h.multusConfig.AddFlags("cni.multus", flagSet)` in `pkg/handlers/lifecycle/handlers.go`.
  - Add `--cni.multus.helm-addon.default-values-template-configmap-name={{ .Values.hooks.cni.multus.helmAddonStrategy.defaultValueTemplateConfigMap.name }}` to `charts/cluster-api-runtime-extensions-nutanix/templates/deployment.yaml`.
  - Remove lines 10–23 (`volumes` and `volumeMounts`) from `charts/cluster-api-runtime-extensions-nutanix/addons/cni/multus/values-template.yaml`.
- [ ] **Step 3**: Run `devbox run -- go test ./pkg/handlers/lifecycle/addons/ ./pkg/handlers/lifecycle/cni/multus/` (exercises `TestAddonValuesTemplates` and `TestHelmChartRenderedYAML` with `yamllint --strict`).

**Completion Criteria**: Both test packages pass; rendered chart YAML passes `yamllint --strict`; `FR-007` and `FR-008` satisfied.

---

### T3 — `MultusHandler` Opt-in, Upgrade Reconciliation, and Custom `values.sourceRef` Safeguard

**Files**:

- Modify: `pkg/handlers/lifecycle/cni/multus/handler_test.go`
- Modify: `pkg/handlers/lifecycle/cni/multus/template.go`
- Modify: `pkg/handlers/lifecycle/cni/multus/handler.go`
- Modify: `pkg/handlers/lifecycle/cni/multus/doc.go`

**Steps**:

- [ ] **Step 1 (Red)**: Add table-driven unit tests in `pkg/handlers/lifecycle/cni/multus/handler_test.go` covering:
  1. New cluster on Docker/AWS/EKS/Nutanix with `cni.multus` set (`Strategy: HelmAddon`) deploys `multus-<uuid>` `HelmChartProxy` with expected socket path (`Cilium`, `Calico`, `Flow`).
  2. New cluster on Nutanix/EKS/Docker/AWS with `cni.multus == nil` and no existing `HelmChartProxy` skips without error and creates no `HelmChartProxy`.
  3. Existing cluster with `cni.multus == nil` and an existing `multus-<uuid>` `HelmChartProxy` reconciles the `HelmChartProxy` on `BeforeClusterUpgrade` without mutating `Cluster`.
  4. Cluster with `cni == nil` skips without error.
  5. `cni.multus` with unsupported/empty strategy returns `ResponseStatusFailure`.
  6. `cni.multus.values.sourceRef` referencing a `ConfigMap` in `cluster.Namespace` without `daemonConfig.readinessIndicatorFile` sets Cluster `OwnerReference` on the `ConfigMap`, preserves custom keys (`nodeSelector`, `daemonConfig.logLevel`), and injects `daemonConfig.readinessIndicatorFile`.
  7. `cni.multus.values.sourceRef` with an explicit non-empty `daemonConfig.readinessIndicatorFile` preserves the user override verbatim.
  8. `cni.multus.values.sourceRef` with missing `ConfigMap` or non-map `daemonConfig` returns `ResponseStatusFailure`.
  Run `devbox run -- go test ./pkg/handlers/lifecycle/cni/multus/` and confirm failure.
- [ ] **Step 2 (Green)**:
  - Update `pkg/handlers/lifecycle/cni/multus/template.go` to execute Go template expansion with `.ReadinessSocketPath` and ensure `daemonConfig.readinessIndicatorFile` is populated via `sigs.k8s.io/yaml` whenever unset/empty in the rendered YAML (returning `rendered` verbatim when already populated so default template comments/formatting stay intact).
  - Update `pkg/handlers/lifecycle/cni/multus/handler.go` to:
    - Remove the `eks`/`nutanix` provider check.
    - When `cniVar.Multus == nil`, look up `HelmChartProxy` `multus-<cluster-uuid>` in `cluster.Namespace`; skip if `NotFound`, reconcile with default values if present.
    - When `cniVar.Multus != nil`, validate `cniVar.Multus.Strategy == v1alpha1.AddonStrategyHelmAddon` and support `cniVar.Multus.Values.SourceRef` (setting Cluster `OwnerReference` via `handlersutils.EnsureClusterOwnerReferenceForObject` and switching `targetNamespace` to `cluster.Namespace`).
  - Update `pkg/handlers/lifecycle/cni/multus/doc.go` to reflect opt-in behavior across all providers.
- [ ] **Step 3**: Run `devbox run -- go test -race ./pkg/handlers/lifecycle/cni/...` and confirm all tests pass.

**Completion Criteria**: All unit tests pass with `-race`; `FR-003`–`FR-006` and `SC-002` satisfied.

---

### T4 — Pre-populated Examples and E2E Readiness Helper

**Files**:

- Modify: `hack/examples/patches/cilium.yaml`
- Modify: `hack/examples/patches/calico.yaml`
- Modify: `hack/examples/patches/nutanix-flow.yaml`
- Modify: `hack/examples/patches/eks/initialize-variables.yaml`
- Regenerate: `examples/` via `make examples.sync`
- Modify: `test/e2e/cni_helpers.go`

**Steps**:

- [ ] **Step 1**: Add `multus: {}` under `cni` in `hack/examples/patches/cilium.yaml`, `hack/examples/patches/calico.yaml`, `hack/examples/patches/nutanix-flow.yaml`, and `hack/examples/patches/eks/initialize-variables.yaml`.
- [ ] **Step 2**: Run `devbox run -- make examples.sync` and verify generated `examples/capi-quick-start/*.yaml` include `multus: {}` under `addons.cni` and `charts/.../defaultclusterclasses/` have no unexpected topology mutation changes.
- [ ] **Step 3**: Update `WaitForCNIToBeReadyInWorkloadCluster` in `test/e2e/cni_helpers.go` so that when `input.CNI.Multus != nil`, it waits for the `multus` `HelmReleaseProxy` (for `AddonStrategyHelmAddon`) and the `kube-system/multus` `DaemonSet` on the workload cluster.
- [ ] **Step 4**: Verify e2e package compiles cleanly via `devbox run -- go test -tags=e2e -run=^$ ./test/e2e/...`.

**Completion Criteria**: `examples/` synced cleanly; `test/e2e` compiles with `-tags=e2e`; `FR-009` and `FR-010` satisfied.

---

### T5 — User Documentation (`docs/content/addons/cni.md`)

**Files**:

- Modify: `docs/content/addons/cni.md`

**Steps**:

- [ ] **Step 1**: Add a `## Multus` section to `docs/content/addons/cni.md` covering:
  - Enabling Multus via `addons.cni.multus: {}` (`HelmAddon` strategy) alongside `Cilium`, `Calico`, or `Flow`.
  - Opting out before cluster creation by omitting/removing `multus` from `addons.cni` in generated manifests.
  - Customizing Multus Helm values via `addons.cni.multus.values.sourceRef` (`ConfigMap` with `values.yaml`), noting that CAREN automatically populates `daemonConfig.readinessIndicatorFile` for the primary CNI unless explicitly overridden.
  - Custom Cilium values note (`cni.exclusive: false` required so Cilium does not overwrite Multus CNI config).
  - Existing-cluster upgrade behavior and day-2 removal (`kubectl delete helmchartproxy multus-<cluster-uuid>` with `addons.cni.multus` omitted).
- [ ] **Step 2**: Lint documentation with `devbox run -- prek run --files docs/content/addons/cni.md`.

**Completion Criteria**: `docs/content/addons/cni.md` passes markdownlint and license checks; `FR-011` satisfied.

---

### T6 — Full Verification & Spec Audit

**Steps**:

- [ ] **Step 1**: Run `devbox run -- make fmt`, `devbox run -- make lint`, `devbox run -- make test`, and `devbox run -- make pre-commit`.
- [ ] **Step 2**: Audit `git status` and `git diff` against every requirement (`FR-001`–`FR-012`, `SC-001`–`SC-003`) in `spec.md`, confirming zero edits under `pkg/handlers/*/mutation/` and no unintended file modifications.

**Completion Criteria**: All quality gates green; self-review surfaces no blockers or should-fix issues.
