<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Implementation Plan: Expose `imageMaximumGCAge` and `imageMinimumGCAge` in KubeletConfiguration

**Branch**: `NCN-119707-kubelet-image-gc-age` | **Date**: 2026-11-25 | **Spec**: [./spec.md](./spec.md)
**Input**: Feature specification from `.specify/features/NCN-119707/spec.md`

## Summary

Expose `imageMinimumGCAge` and `imageMaximumGCAge` (`*metav1.Duration`) on `v1alpha1.KubeletConfiguration` so operators can declaratively configure persistent kubelet image garbage collection age policies on both worker pools (`workerConfig.kubeletConfiguration`) and control plane nodes (`clusterConfig.controlPlane.kubeletConfiguration`). Enforce upstream `ValidateKubeletConfiguration` constraints in both CRD/OpenAPI CEL rules and the `Cluster` admission webhook (`imageMinimumGCAge > 0s`, `imageMaximumGCAge >= 0s`, and `imageMaximumGCAge > effectiveImageMinimumGCAge` using kubelet's `2m` default when `imageMinimumGCAge` is unset). Render both fields conditionally in `kubeletconfigpatch.yaml.tmpl` with zero change to `GeneratePatches` output when unset.

## Technical Context

**Language/Version**: Go 1.26 (toolchain go1.27.3 per `go.mod`)
**Primary Dependencies**: `k8s.io/apimachinery` (`metav1.Duration`), `k8s.io/kubelet/config/v1beta1`, `sigs.k8s.io/cluster-api`, `sigs.k8s.io/controller-runtime`
**Storage**: N/A (Cluster API topology variables and kubeadm strategic merge patch files)
**Testing**: `testify` unit tests, `capitest.ValidateDiscoverVariables` OpenAPI/CEL tests, Ginkgo/Gomega webhook and `GeneratePatches` tests
**Target Platform**: Kubernetes management cluster (CAREN Runtime Extension + admission webhook)
**Project Type**: Multi-module Go repository (`api/`, `common/`, root module)
**Constraints**: Handler Version Safety (Constitution VII) — `GeneratePatches` output MUST remain byte-identical when new fields are nil; no new dependencies (Constitution VI); line length <= 120 chars; import aliases per `.golangci.yml`
**Scale/Scope**: `api/v1alpha1`, `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration`, `pkg/webhook/cluster`, `docs/content/customization/kubeadm/kubelet-configuration.md`

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| I. API-First | Pass | Types and CEL rules live in `api/v1alpha1/kubelet_types.go`; regenerated via `make go-generate`. |
| II. Handler-per-Provider | Pass | Shared kubeadm mutation logic stays in `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/`. |
| III. Library-First | Pass | No `cmd/` changes; logic lives in `api/` and `pkg/`. |
| IV. Tests Required | Pass | Unit (`testify`), OpenAPI/CEL (`capitest`), and Ginkgo webhook/patch tests included via TDD. |
| V. Code Style | Pass | Enforced via `make fmt`, `make lint`, and `make pre-commit`. |
| VI. Dependency Management | Pass | Zero new dependencies (`metav1.Duration` already used on `KubeletConfiguration`). |
| VII. Handler Version Safety | Pass | Conditional `{{- with .ImageMinimumGCAge }}` / `{{- with .ImageMaximumGCAge }}` emits nothing when unset; existing cluster patches are byte-identical. |
| VIII. Handler Documentation | Pass | `docs/content/customization/kubeadm/kubelet-configuration.md` updated in the same change. |

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-119707/
├── spec.md
└── plan.md
```

### Source Code (repository root)

```text
api/v1alpha1/
├── kubelet_types.go                  # ImageMinimumGCAge, ImageMaximumGCAge + CEL XValidation markers
├── kubelet_types_test.go             # IsEmpty tests covering new fields
├── zz_generated.deepcopy.go          # Regenerated via make go-generate
└── crds/*.yaml                       # Regenerated via make go-generate

pkg/webhook/cluster/
├── kubeletconfiguration_validator.go      # Image GC age validation with 2m default min-age handling
└── kubeletconfiguration_validator_test.go # Ginkgo tests for CP, worker default, and MD override paths

pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/
├── kubelet_config.go                      # Template input fields + Duration.String() mapping
├── embedded/kubeletconfigpatch.yaml.tmpl  # Conditional YAML rendering for imageMinimumGCAge/imageMaximumGCAge
├── kubelet_config_test.go                 # Deserialization tests into kubeletconfigv1beta1.KubeletConfiguration
├── variables_test.go                      # OpenAPI + CEL validation tests for Docker, AWS, Nutanix
├── inject_worker_test.go                  # Worker GeneratePatches integration test
└── inject_controlplane_test.go            # Control plane GeneratePatches integration test

docs/content/customization/kubeadm/
└── kubelet-configuration.md               # User-facing docs, examples, rollout & /configz verification
```

---

## Atomic Tasks (TDD)

### Task 1: API Fields, CEL Validation, and Variable Schema Tests (5 min)

**Files:**
- Modify: `api/v1alpha1/kubelet_types_test.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/variables_test.go`
- Modify: `api/v1alpha1/kubelet_types.go`
- Regenerate: `api/v1alpha1/zz_generated.deepcopy.go`, `api/v1alpha1/crds/*.yaml` (`make go-generate`)

**Steps:**
1. **Red:** Add `IsEmpty` assertions in `api/v1alpha1/kubelet_types_test.go` and variable validation cases in `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/variables_test.go` for:
   - Valid `imageMaximumGCAge: "48h"` alone
   - Valid `imageMinimumGCAge: "15m"` and `imageMaximumGCAge: "48h"`
   - Valid `imageMinimumGCAge: "30s"` and `imageMaximumGCAge: "1m"` (below 2m default when min is explicitly lowered)
   - Valid `imageMaximumGCAge: "0s"` (disabled)
   - Invalid `imageMinimumGCAge: "0s"` and negative `imageMinimumGCAge: "-1m"`
   - Invalid negative `imageMaximumGCAge: "-1m"`
   - Invalid `imageMaximumGCAge: "1m"` with `imageMinimumGCAge` unset (`<= 2m` default)
   - Invalid `imageMaximumGCAge: "2m"` with `imageMinimumGCAge` unset (`<= 2m` default)
   - Invalid `imageMinimumGCAge: "30m"` with `imageMaximumGCAge: "15m"` (`max <= min`)
   - Invalid duration format `"2d"` (raw map input)
2. Prove tests fail to compile (`ImageMinimumGCAge` / `ImageMaximumGCAge` undefined).
3. **Green:** Add `ImageMinimumGCAge *metav1.Duration` and `ImageMaximumGCAge *metav1.Duration` to `v1alpha1.KubeletConfiguration` with field-level and struct-level `+kubebuilder:validation:XValidation` rules:
   - Field `ImageMinimumGCAge`: `rule="duration(self) > duration('0s')"`
   - Field `ImageMaximumGCAge`: `rule="duration(self) >= duration('0s')"`
   - Struct cross-field when `imageMinimumGCAge` is set: `rule="!has(self.imageMaximumGCAge) || !has(self.imageMinimumGCAge) || duration(self.imageMaximumGCAge) == duration('0s') || duration(self.imageMaximumGCAge) > duration(self.imageMinimumGCAge)"`
   - Struct cross-field when `imageMinimumGCAge` is unset: `rule="!has(self.imageMaximumGCAge) || has(self.imageMinimumGCAge) || duration(self.imageMaximumGCAge) == duration('0s') || duration(self.imageMaximumGCAge) > duration('2m')"`
4. Run `devbox run -- make go-generate` to regenerate deepcopy and CRDs.
5. Run `api` unit tests and `TestVariableValidation_*` in `kubeletconfiguration` to verify PASS.

**Completion Criteria:** `api/v1alpha1` tests and `TestVariableValidation_{Docker,AWS,Nutanix}` pass; generated CRDs contain both fields and CEL rules.

---

### Task 2: Admission Webhook Validation (5 min)

**Files:**
- Modify: `pkg/webhook/cluster/kubeletconfiguration_validator_test.go`
- Modify: `pkg/webhook/cluster/kubeletconfiguration_validator.go`

**Steps:**
1. **Red:** Add Ginkgo specs in `pkg/webhook/cluster/kubeletconfiguration_validator_test.go` testing `kubeletConfigurationValidator` against:
   - `imageMaximumGCAge: 48h` accepted
   - `imageMaximumGCAge: 0s` accepted
   - `imageMinimumGCAge: 30s` + `imageMaximumGCAge: 1m` accepted
   - `imageMinimumGCAge: 0s` rejected with field path
   - `imageMinimumGCAge: -1m` rejected with field path
   - `imageMaximumGCAge: -1m` rejected with field path
   - `imageMaximumGCAge: 1m` (with `imageMinimumGCAge` nil) rejected citing `2m0s` default `imageMinimumGCAge`
   - `imageMinimumGCAge: 30m` + `imageMaximumGCAge: 30m` on a worker `MachineDeployment` override rejected with `workers.machineDeployments["md-0"].variables.overrides[workerConfig].kubeletConfiguration` path
2. Run webhook tests and prove the new rejection cases fail.
3. **Green:** Implement `validateImageGCAges(cfg *v1alpha1.KubeletConfiguration, path string) error` in `pkg/webhook/cluster/kubeletconfiguration_validator.go` (using `defaultImageMinimumGCAge = 2 * time.Minute`) and call it from `validateKubeletConfig`.
4. Re-run `pkg/webhook/cluster` tests and verify PASS.

**Completion Criteria:** All `KubeletConfigurationValidator` specs pass.

---

### Task 3: Strategic Merge Patch Rendering & Mutation Handler Tests (5 min)

**Files:**
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config_test.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_worker_test.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/inject_controlplane_test.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/kubelet_config.go`
- Modify: `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/embedded/kubeletconfigpatch.yaml.tmpl`

**Steps:**
1. **Red:** Add unit tests in `kubelet_config_test.go` (`TestRenderKubeletConfigPatch_ImageMinimumGCAge`, `TestRenderKubeletConfigPatch_ImageMaximumGCAge`, `TestRenderKubeletConfigPatch_ImageGCAgesTogether`, `TestRenderKubeletConfigPatch_ImageMaximumGCAgeZero`, and assert zero values when unset) plus worker and control-plane `GeneratePatches` test cases in `inject_worker_test.go` and `inject_controlplane_test.go`.
2. Run `kubeletconfiguration` tests and prove the new tests fail because the fields are not rendered yet.
3. **Green:** Add `ImageMinimumGCAge` and `ImageMaximumGCAge` to `kubeletConfigTemplateInput`, populate them via `.Duration.String()` in `toTemplateInput`, and add conditional blocks to `embedded/kubeletconfigpatch.yaml.tmpl`.
4. Re-run `kubeletconfiguration` tests (including envtest-backed `inject_*_test.go`) and verify PASS, including all pre-existing patch golden expectations.

**Completion Criteria:** All `pkg/handlers/generic/mutation/kubeadm/kubeletconfiguration/...` unit and integration tests pass; existing patch tests remain untouched and green.

---

### Task 4: User Documentation (5 min)

**Files:**
- Modify: `docs/content/customization/kubeadm/kubelet-configuration.md`

**Steps:**
1. Add `imageMinimumGCAge` and `imageMaximumGCAge` to the Supported options table in `docs/content/customization/kubeadm/kubelet-configuration.md`.
2. Expand the `### Image garbage collection` section to cover:
   - Threshold-based GC (`imageGCHighThresholdPercent`, `imageGCLowThresholdPercent`) vs age-based GC (`imageMaximumGCAge`) and minimum-age protection (`imageMinimumGCAge`).
   - Safety guarantee: kubelet image GC only removes unused images and never deletes images referenced by running or active containers.
   - Validation constraint (`imageMaximumGCAge` must be greater than `imageMinimumGCAge`, which defaults to `"2m"`, and durations use Go units up to hours such as `"48h"` rather than `"2d"`).
   - Rollout and verification: changing `kubeletConfiguration` updates the `KubeadmConfigTemplate` / `KubeadmControlPlaneTemplate` and triggers a rolling node replacement; show `kubectl` commands to check CAPI rollout status and inspect effective node kubelet config via `/api/v1/nodes/<node>/proxy/configz`.
3. Verify markdown formatting conforms to `.markdownlint.json`.

**Completion Criteria:** Documentation covers FR-007 and passes `make pre-commit`.

---

### Task 5: Full Audit, Quality Gates, Self-Review, and Signed Commit (5 min)

**Steps:**
1. Audit every FR and SC in `.specify/features/NCN-119707/spec.md` against the diff; verify zero scope creep and zero handler version changes.
2. Run `devbox run -- make fmt`, `devbox run -- make lint`, `devbox run -- make pre-commit`, and targeted + module tests (`devbox run -- make test`).
3. Perform substantive self-review of `git diff` and `git status`.
4. Stage intended files and create a 1Password-signed conventional commit (`feat: [NCN-119707] Add kubelet image GC age configuration` using `required_permissions: ["all"]`) and verify `gpgsig` header on `HEAD`.

**Completion Criteria:** All tests, linters, and pre-commit hooks pass; working tree clean; commit is signed and ready for review.
