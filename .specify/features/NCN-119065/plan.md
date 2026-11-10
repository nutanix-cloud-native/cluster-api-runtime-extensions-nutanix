<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Service Account Issuer Configuration for API Server

**Branch**: `NCN-119065-service-account-issuer` | **Date**: 2026-11-10 | **Spec**: [spec.md](./spec.md) | **Jira**: [NCN-119065](https://jira.nutanix.com/browse/NCN-119065)

**Goal**: Allow operators to configure the API server's ServiceAccount token issuer (workload identity) by declaring `clusterConfig.serviceAccountIssuer` on `KubeadmClusterConfigSpec`; a provider-agnostic mutation handler renders `service-account-issuer`, `api-audiences`, and `service-account-jwks-uri` as `KubeadmControlPlaneTemplate.spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs`. Opt-in / no-op when unset.

**Architecture**: Owned API type `ServiceAccountIssuer` in `api/v1alpha1/serviceaccountissuer_types.go` (pointer on `KubeadmClusterConfigSpec.serviceAccountIssuer`), new mutation handler `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/` that follows the `extraapiservercertsans` / `podsecurityadmission` pattern (`patches.MutateIfApplicable` on `selectors.ControlPlane()` + `variables.Get`), registered in `pkg/handlers/generic/mutation/handlers.go:MetaMutators()` before `sortextraargs`, kubebuilder + CEL validation on the CRD, deterministic `extraArgs` via existing `sortextraargs` stage.

**Tech Stack**: Go 1.22+; kubebuilder/controller-gen + CEL (`XValidation`); CAPI topology mutation (`common/pkg/capi/clustertopology/{handlers/mutation,patches,selectors}` + `variables`); `bootstrapv1.Arg{Name, Value *string}` on `KubeadmControlPlaneTemplate`; `testify` for API tests, `Ginkgo`/`Gomega` + `capitest` for handler tests; `make go-generate` for CRDs/deepcopy.

---

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | Types live in `api/v1alpha1/` only; no business logic in API module. |
| II. Handler-per-Provider | Pass | Handler in `generic/kubeadm` (provider-agnostic), no cross-provider imports; scoped to kubeadm providers only (documented — EKS excluded). |
| III. Library-First | Pass | No `cmd/` changes; wiring is handler registration only. |
| IV. Tests Required | Pass | API validation tests + handler Ginkgo suite (no-op, single issuer, audiences, jwksURI, conflict error, coexistence). |
| V. Code Style | Pass | Import aliases per `.golangci.yml:importas`; `make fmt`/`make lint` gating. |
| VI. Dependency Management | Pass | No new direct deps; only `bootstrapv1.Arg` + existing patch helpers. |
| VII. Handler Version Safety | Pass* | Opt-in pointer field; `GeneratePatches` output for inputs not setting `serviceAccountIssuer` is byte-identical to pre-change (verified by omitted-variable test). No handler version bump required under current `main`. *If `origin/NCN-107121` versioning lands before merge, re-evaluate — plan notes fallback (see Risks). |
| VIII. Handler Documentation | Pass | New page `docs/content/customization/kubeadm/service-account-issuer.md` + index link in same PR. |

---

## Research Notes

*No separate `research.md` needed — scope is minimal and precedent is established. Summary of confirmed precedents:*

- **API validation precedent**: `api/v1alpha1/clusterconfig_types.go:365-378` uses `// +kubebuilder:validation:Format=uri` + `Pattern=^https?://` for registry mirror URLs; `api/v1alpha1/nutanix_clusterconfig_types.go:32-33` uses `Format=uri` + `Pattern=^https://` for an `https://`-only field — this is the pattern for `issuer`/`jwksURI`.
- **Handler shape**: `pkg/handlers/generic/mutation/kubeadm/extraapiservercertsans/inject.go:100-120` and `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/inject.go:108-130` both use `variables.Get[T](vars, v1alpha1.ClusterConfigVariableName, VariableName)` + `IsNotFoundError` no-op + `nil` guard + `patches.MutateIfApplicable(..., selectors.ControlPlane(), ...)`. `ServiceAccountIssuer` handler follows the same shape, but writes `apiServer.extraArgs` (no Files/Volumes).
- **Control-plane-only mutation**: all existing kubeadm mutation handlers use `selectors.ControlPlane()` → `KubeadmControlPlaneTemplate`. No worker patch.
- **Sorting / stability**: `pkg/handlers/generic/mutation/kubeadm/sortextraargs` sorts `apiServer.extraArgs` by name. New handler must be registered **before** `sortextraargs` in `MetaMutators()` so final output is stable (FR-008). Handler itself does not sort; it appends and lets `sortextraargs` normalize.
- **EKS exclusion**: `pkg/handlers/generic/mutation/handlers.go` today has only generic mutators (no EKS-specific `MetaMutators` path for this concern). Document scope as kubeadm-only (FR-009); no EKS wiring.
- **NCN-118972 coexistence**: that feature's handler touches `--authentication-config`/`--oidc-*` flags + files/volumes at `/etc/kubernetes/authentication.yaml`. No flag overlap — the two handlers compose additively. Plan explicitly excludes `authentication-config` from conflict checks; handler guards only its own three flags.
- **NCN-107121 versioning**: that stack introduces `pkg/handlers/vN/` + golden-output freeze checks. On current `main` (pre-merge), opt-in no-op is sufficient. If versioning merges first, the plan's fallback is to cut a version tree (copy current `pkg/handlers/generic/mutation/kubeadm/*` into `pkg/handlers/v2/` or next, bump handler name, dual-register). No need to pre-cut now.

---

## Data Model

Single owned type + pointer on `KubeadmClusterConfigSpec`:

```go
// api/v1alpha1/serviceaccountissuer_types.go

// ServiceAccountIssuer configures the API server's ServiceAccount token issuer.
// When nil on KubeadmClusterConfigSpec, no issuer flags are set (no-op).
// Maps to --service-account-issuer, --api-audiences, --service-account-jwks-uri.
type ServiceAccountIssuer struct {
  // Issuer is the identifier asserted in ServiceAccount JWT `iss` claims.
  // Must be an https:// URL. Triggers --service-account-issuer.
  // +kubebuilder:validation:Required
  // +kubebuilder:validation:Format=uri
  // +kubebuilder:validation:Pattern=`^https://`
  // +kubebuilder:validation:MinLength=1
  // +kubebuilder:validation:MaxLength=2048
  Issuer string `json:"issuer"`

  // Audiences is an optional list of audiences tokens are bound to.
  // When set, rendered as comma-joined --api-audiences in declared order.
  // When absent, --api-audiences is not set (API server defaults to issuer URL).
  // +kubebuilder:validation:Optional
  // +kubebuilder:validation:MaxItems=50
  // +kubebuilder:validation:XValidation:rule="self.all(x, size(x) > 0 && size(x) <= 253)",message="each audience must be 1-253 characters"
  // +kubebuilder:validation:items:MinLength=1
  // +kubebuilder:validation:items:MaxLength=253
  // +listType=set  (unique items — enforced by XListType or CEL; choose one)
  Audiences []string `json:"audiences,omitempty"`

  // JWKSURI overrides the JWKS URI advertised in the OIDC discovery document.
  // When set, must be an https:// URL. Triggers --service-account-jwks-uri.
  // +kubebuilder:validation:Optional
  // +kubebuilder:validation:Format=uri
  // +kubebuilder:validation:Pattern=`^https://`
  // +kubebuilder:validation:MinLength=1
  // +kubebuilder:validation:MaxLength=2048
  JWKSURI string `json:"jwksURI,omitempty"`
}

// In api/v1alpha1/clusterconfig_types.go — add to KubeadmClusterConfigSpec:
  // ServiceAccountIssuer configures the API server's ServiceAccount token issuer.
  // When unset, no issuer flags are set (API server uses kubeadm defaults).
  // +kubebuilder:validation:Optional
  ServiceAccountIssuer *ServiceAccountIssuer `json:"serviceAccountIssuer,omitempty"`
```

**Validation / design notes**:

- `Issuer` required when block present (pointer semantics give opt-in; `Issuer` itself is `Required` non-pointer string — empty string rejected by `MinLength=1` + `Pattern`).
- `Audiences` — `MaxItems=50`, each `1–253`, unique (`+listType=set` preferred over bespoke CEL; verify controller-gen supports it on `string` lists — fallback to CEL `self.all(x, self.exists_one(y, x == y))` if needed).
- `JWKSURI` — `omitempty`; when empty, field is absent and not set; when non-empty, validated as `https://` URI.
- JSON tag `jwksURI` preserves spec casing (spec: `jwksURI`); repo precedent for URI fields uses `URL` (all-caps) but this is a leaf field on an owned type — either is acceptable as long as it matches spec and CRD schema. Prefer `jwksURI` to match spec/FR-004 verbatim.
- No `Format=binary` / `byte` concerns; pure string validation.
- CEL vs OpenAPI: `Pattern` + `Format=uri` gives immediate admission rejection per FR-007; CEL on `audiences` length/uniqueness covers edge cases without webhook.

## File Structure

```text
api/v1alpha1/
├── serviceaccountissuer_types.go        # CREATE: ServiceAccountIssuer type + markers
├── serviceaccountissuer_types_test.go   # CREATE: testify sanity (optional, mirrors authentication/ kubelet tests)
├── clusterconfig_types.go               # MODIFY: add ServiceAccountIssuer *ServiceAccountIssuer to KubeadmClusterConfigSpec
├── zz_generated.deepcopy.go             # GENERATED (make go-generate)
└── crds/caren.nutanix.com_kubeadmclusterconfigs.yaml  # GENERATED (+ siblings for AWS/Docker/Nutanix if shared schema propagates)

pkg/handlers/generic/mutation/kubeadm/
├── serviceaccountissuer/
│   ├── doc.go                           # CREATE: package doc
│   ├── inject.go                        # CREATE: handler (NewPatch, Mutate, helpers)
│   └── inject_test.go                   # CREATE: Ginkgo suite (FR-005/006 + no-op + audiences + jwksURI + conflict + coexistence)
├── podsecurityadmission/inject.go       # READ-ONLY reference
├── extraapiservercertsans/inject.go     # READ-ONLY reference
└── sortextraargs/                       # READ-ONLY — ordering guarantee

pkg/handlers/generic/mutation/handlers.go  # MODIFY: register serviceaccountissuer.NewPatch() in MetaMutators() before sortextraargs

docs/content/customization/kubeadm/
├── service-account-issuer.md            # CREATE: user-facing docs per FR-010
└── _index.md                            # MODIFY: link new page

.specify/features/NCN-119065/
├── spec.md                              # (exists)
├── plan.md                              # (this file)
└── tasks.md                             # (next — optional)

charts/ + examples/                       # GENERATED only via make go-generate / make examples.sync — expect clean aside from CRD deepcopy unless schema touches ClusterClass defaults (it should not; opt-in pointer)
```

**Structure Decision**: Single new handler package mirroring `podsecurityadmission` / `encryptionatrest` — provider-agnostic, isolated, testable. No change to provider-specific handlers or `hack/examples/overlays/clusterclasses` (opt-in feature — default ClusterClasses carry no `serviceAccountIssuer`).

---

## Tasks (TDD-ordered outline — detailed tasks.md follows)

### Task 1: API types — `ServiceAccountIssuer` + plumbing

- [ ] Write failing test in `api/v1alpha1/serviceaccountissuer_types_test.go` (or extend `common_types_test.go`) — marshal round-trip / nil vs zero distinction.
- [ ] Create `api/v1alpha1/serviceaccountissuer_types.go` with markers above; add pointer field to `KubeadmClusterConfigSpec` in `clusterconfig_types.go`.
- [ ] `make go-generate` → deepcopy + CRDs regenerated; verify `charts/` + `examples/` diff is only CRD propagation (expected for shared schema).
- [ ] `go test ./api/v1alpha1 -run TestServiceAccountIssuer` → PASS.

### Task 2: Mutation handler — extraArgs + conflict detection

- [ ] Write failing Ginkgo suite in `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/inject_test.go`:
  - omitted variable → zero patches (no-op, FR-001/SC-003)
  - `issuer` only → `service-account-issuer` arg added
  - `issuer` + `audiences` (single, multi, order preserved, absent → no flag)
  - `jwksURI` present → `service-account-jwks-uri` added; absent → not set
  - pre-existing `service-account-issuer` / `api-audiences` / `service-account-jwks-uri` in `extraArgs` → error with remediation message (FR-006)
  - coexistence with other handlers (e.g. existing `extraArgs` for unrelated flags are preserved)
  - removal (nil/empty audiences) → no-op, no active deletion (edge case per spec)
- [ ] Implement `inject.go`:
  ```go
  const (
    VariableName = "serviceAccountIssuer"
    ServiceAccountIssuerArg = "service-account-issuer"
    APIAudiencesArg         = "api-audiences"
    ServiceAccountJWKSURIArg = "service-account-jwks-uri"
  )
  type handler struct { variableName, variableFieldPath string }
  func NewPatch() *handler { return newHandler(v1alpha1.ClusterConfigVariableName, VariableName) }
  func (h *handler) Mutate(ctx, obj, vars, holderRef, clusterKey, clusterGetter) error
  ```
  - `variables.Get[*v1alpha1.ServiceAccountIssuer](vars, ...)`; `IsNotFound → no-op`; `nil → no-op`.
  - Inspect `obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs`; if any entry has `Name` in `{service-account-issuer, api-audiences, service-account-jwks-uri}` → return error `... is mutually exclusive with serviceAccountIssuer; remove the direct extraArgs and use clusterConfig.serviceAccountIssuer instead`.
  - Else append/update `extraArgs` entries: `service-account-issuer` always; `api-audiences` as `strings.Join(audiences, ",")` if non-empty; `service-account-jwks-uri` if non-empty. Use `Value: &v` form (`bootstrapv1.Arg{Name, Value *string}`).
  - No file/volume handling; do not touch `CertSANs`/`Files`/`ExtraVolumes`.
- [ ] `go test ./pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer -count=1` → PASS.

### Task 3: Wiring — register handler

- [ ] In `pkg/handlers/generic/mutation/handlers.go`, import `serviceaccountissuer` and add `serviceaccountissuer.NewPatch()` to `MetaMutators()` return slice **before** `sortextraargs.NewPatch()` (FR-008 ordering). No change to `ControlPlaneMetaMutators`/`WorkerMetaMutators` — this handler lives in `MetaMutators` (generic) pipeline, matching `extraapiservercertsans` / `podsecurityadmission`.
- [ ] `go vet ./pkg/handlers/generic/mutation/...` + `go test ./pkg/handlers/generic/mutation/... -count=1` smoke; verify omitted-variable case still produces no patches.

### Task 4: Docs — customization page + index

- [ ] Create `docs/content/customization/kubeadm/service-account-issuer.md` per FR-010: what it does (workload identity via OIDC discovery), provider scoping (kubeadm-only, EKS excluded), minimum K8s version note, configuration table (`issuer` / `audiences` / `jwksURI`), single-issuer example, audiences example, jwksURI example, rollout/validity note (“changing issuer triggers control-plane rollout; prior tokens remain valid until expiry”), conflict note (`extraArgs` vs typed field).
- [ ] Update `docs/content/customization/kubeadm/_index.md` (or `docs/content/customization/_index.md` — verify on branch) to link new page.
- [ ] `make preview-docs` smoke — page renders without warnings.

### Task 5: Final verification gate

- [ ] `make go-generate` clean (no diff beyond `api/` CRDs/deepcopy); `git status` shows no unintended `charts/`/`examples/` changes or only expected CRD deepcopy.
- [ ] `make fmt` + `make lint` + `make pre-commit` clean (license headers current year).
- [ ] `make test` (or targeted `make test.root` + `go test ./api/v1alpha1` + handler suite) — green.
- [ ] Manual SC-003 spot-check: `GeneratePatches` with no `serviceAccountIssuer` set produces byte-identical output before/after (no version bump).

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| CEL `+listType=set` on `Audiences` not supported for string lists in controller-gen | Fall back to `XValidation` uniqueness rule or rely on OpenAPI `uniqueItems`; verify via `make go-generate` output — if CRD lacks uniqueness, add CEL `self.all(x, self.exists_one(y, x == y))`. |
| `sortextraargs` ordering assumption violated (handler registered after sort) | Explicitly register before `sortextraargs` in `MetaMutators()`; existing tests in `sortextraargs/inject_test.go` confirm sorting of unsorted `apiServer.extraArgs`. |
| Composition with `authentication` (NCN-118972) if it merges first | No flag overlap (that feature uses `authentication-config` + `--oidc-*`); handler conflict check covers only its own three flags; coexistence test confirms additive `ExtraArgs`. |
| NCN-107121 versioning lands and freeze-check rejects new handler without version cut | Keep opt-in no-op and verify SC-003; if freeze check triggers, cut version tree (fallback in Research Notes) and dual-register — additive, no SC-003 violation. |
| CRD `MaxLength=2048` on `issuer`/`jwksURI` conflicts with URL length reality | Matches RFC 3986 issuer practice and `clusterconfig_types.go:354` precedent; API server also validates at runtime — early rejection is deliberate (FR-007). |

---

## Out of Scope (explicitly not in this plan)

- IdP / Dex / `kube-oidc-proxy` / cloud IAM role bindings (consumer side).
- `--service-account-key-file` / `--service-account-signing-key-file` / `--service-account-extend-token-expiration` tuning.
- End-user OIDC/JWT authentication (`--oidc-*` / `AuthenticationConfiguration`) — separate feature NCN-118972.
- EKS support (no API server flag access).
- E2E against a real cluster (label-driven `test/e2e` later if desired).
