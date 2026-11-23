<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Service Account Issuer Configuration for API Server

**Branch**: `NCN-119065-service-account-issuer` | **Date**: 2026-11-10 | **Spec**: [spec.md](./spec.md) | **Jira**: [NCN-119065](https://jira.nutanix.com/browse/NCN-119065)

**Goal**: Allow operators to configure the API server's ServiceAccount token issuer (workload identity) by declaring `clusterConfig.serviceAccountIssuer` on `KubeadmClusterConfigSpec`; a provider-agnostic mutation handler renders `service-account-issuer`, alphabetically sorted `api-audiences`, `service-account-jwks-uri`, and `service-account-max-token-expiration` as `KubeadmControlPlaneTemplate.spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs`. Opt-in / no-op when unset, with self-contained provider-agnostic e2e coverage in `test/framework/oidc` and `test/e2e/quick_start_test.go`.

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
| VIII. Handler Documentation | Pass | New page `docs/content/customization/kubeadm/service-account-issuer.md` (auto-indexed by Hugo) in same PR. |

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
// Maps to --service-account-issuer, --api-audiences, --service-account-jwks-uri,
// --service-account-max-token-expiration.
type ServiceAccountIssuer struct {
  // Issuer is the identifier asserted in ServiceAccount JWT iss claims.
  // Must be an https:// URL without userinfo, query, or fragment. Triggers --service-account-issuer.
  // +kubebuilder:validation:Required
  // +kubebuilder:validation:Format=uri
  // +kubebuilder:validation:Pattern=`^https://[^/?#@\s]+(/[^\s?#]*)?$`
  // +kubebuilder:validation:XValidation:rule="isURL(self) && url(self).getHostname() != ''",message="must be a valid https URL with a non-empty hostname"
  // +kubebuilder:validation:MinLength=9
  // +kubebuilder:validation:MaxLength=2048
  Issuer string `json:"issuer"`

  // Audiences is an optional list of audiences tokens are bound to.
  // When set, rendered as comma-joined --api-audiences sorted alphabetically.
  // When absent, --api-audiences is not set (API server defaults to issuer URL).
  // +kubebuilder:validation:Optional
  // +kubebuilder:validation:MaxItems=50
  // +kubebuilder:validation:UniqueItems=true
  // +kubebuilder:validation:items:MinLength=1
  // +kubebuilder:validation:items:MaxLength=253
  // +kubebuilder:validation:items:Pattern=`^[^\s,"]+$`
  Audiences []string `json:"audiences,omitempty"`

  // JWKSURI overrides the JWKS URI advertised in the OIDC discovery document.
  // When set, must be an https:// URL without userinfo or fragment. Triggers --service-account-jwks-uri.
  // +kubebuilder:validation:Optional
  // +kubebuilder:validation:Format=uri
  // +kubebuilder:validation:Pattern=`^https://[^/?#@\s]+(/[^\s#]*)?$`
  // +kubebuilder:validation:XValidation:rule="isURL(self) && url(self).getHostname() != ''",message="must be a valid https URL with a non-empty hostname"
  // +kubebuilder:validation:MinLength=9
  // +kubebuilder:validation:MaxLength=2048
  JWKSURI string `json:"jwksURI,omitempty"`

  // MaxTokenExpiration is the maximum lifetime for ServiceAccount tokens.
  // Must be between 1h and 2^32 seconds (1193046h28m16s).
  // When set, triggers --service-account-max-token-expiration.
  // +kubebuilder:validation:Optional
  // +kubebuilder:validation:XValidation:rule="duration(self) >= duration('1h') && duration(self) <= duration('1193046h28m16s')",message="must be between 1h and 1193046h28m16s (2^32s)"
  MaxTokenExpiration *metav1.Duration `json:"maxTokenExpiration,omitempty"`
}

// In api/v1alpha1/clusterconfig_types.go — add to KubeadmClusterConfigSpec
// (plus CEL ensuring authentication.jwt[].issuer.url != serviceAccountIssuer.issuer):
  // ServiceAccountIssuer configures the API server's ServiceAccount token issuer.
  // When unset, no issuer flags are set (API server uses kubeadm defaults).
  // +kubebuilder:validation:Optional
  ServiceAccountIssuer *ServiceAccountIssuer `json:"serviceAccountIssuer,omitempty"`
```

**Validation / design notes**:

- `Issuer` required when block present (`Required`, `Format=uri`, no userinfo/query/fragment `Pattern`, CEL `isURL(self) && url(self).getHostname() != ''`, `MinLength=9`, `MaxLength=2048`). Validated in handler via upstream `k8s.io/apiserver/pkg/util/webhook.ValidateWebhookURL` + `k8s.io/apimachinery/pkg/util/validation.IsValidPortNum`.
- `Audiences` — `MaxItems=50`, each `1–253`, `UniqueItems=true`, `items:Pattern=^[^\s,"]+$` (forbids whitespace, commas, and quotes so `pflag.StringSlice` CSV parsing cannot split or crash), sorted alphabetically before comma-joining, and verified via upstream `github.com/spf13/pflag.NewFlagSet`.
- `JWKSURI` — `omitempty`; when set, validated as `https://` URL with non-empty hostname, no userinfo or fragment (query parameters allowed per OpenID Discovery 1.0 Section 3).
- `MaxTokenExpiration` — `*metav1.Duration` bounded to `[1h, 1193046h28m16s]` (`[1h, 2^32s]`), matching `kube-apiserver`'s startup validation in `ServiceAccountTokenOptions.Validate()`.
- Cross-field validation: `KubeadmClusterConfigSpec` CEL and handler validation reject `serviceAccountIssuer.issuer` overlapping any `authentication.jwt[].issuer.url` (`kube-apiserver` `disallowedIssuers`).

## File Structure

```text
api/v1alpha1/
├── serviceaccountissuer_types.go        # CREATE: ServiceAccountIssuer type + markers
├── serviceaccountissuer_types_test.go   # CREATE: testify round-trip / omitted / nil tests
├── clusterconfig_types.go               # MODIFY: add ServiceAccountIssuer + non-overlapping issuer CEL on KubeadmClusterConfigSpec
├── zz_generated.deepcopy.go             # GENERATED (make go-generate)
└── crds/caren.nutanix.com_*clusterconfigs.yaml  # GENERATED

pkg/handlers/generic/mutation/kubeadm/
├── serviceaccountissuer/
│   ├── doc.go                           # CREATE: package doc
│   ├── inject.go                        # CREATE: handler + upstream webhook/pflag validation + idempotent ensureArg
│   ├── inject_test.go                   # CREATE: Ginkgo suite (exact ConsistOf matchers, conflicts, idempotency, coexistence)
│   └── variables_test.go                # CREATE: CRD OpenAPI + CEL validation suite across Docker, AWS, Nutanix, and EKS
├── podsecurityadmission/inject.go       # READ-ONLY reference
├── extraapiservercertsans/inject.go     # READ-ONLY reference
└── sortextraargs/                       # READ-ONLY — ordering guarantee

pkg/handlers/generic/mutation/handlers.go  # MODIFY: register serviceaccountissuer.NewPatch() in MetaMutators()

docs/content/customization/kubeadm/
└── service-account-issuer.md            # CREATE: user-facing docs per FR-010 (auto-indexed by Hugo)

test/framework/oidc/
├── oidc.go                              # MODIFY: shared clusterConfig YAML AST helper
├── serviceaccountissuer.go              # CREATE: self-contained discovery, JWKS, TokenRequest, SelfSubjectReview helpers
└── serviceaccountissuer_test.go         # CREATE: unit + envtest kube-apiserver integration tests

test/e2e/
└── quick_start_test.go                  # MODIFY: boot-time template injection + post-provision verification

.specify/features/NCN-119065/
├── spec.md
└── plan.md
```

**Structure Decision**: Single new handler package mirroring `podsecurityadmission` / `authentication` — provider-agnostic, isolated, testable. No change to provider-specific handlers or `hack/examples/overlays/clusterclasses` (opt-in feature — default ClusterClasses carry no `serviceAccountIssuer`).

---

## Tasks

### Task 1: API types — `ServiceAccountIssuer` + plumbing

- [x] Write failing test in `api/v1alpha1/serviceaccountissuer_types_test.go` — marshal round-trip / nil vs zero distinction.
- [x] Create `api/v1alpha1/serviceaccountissuer_types.go` with markers above; add pointer field and non-overlapping issuer CEL to `KubeadmClusterConfigSpec` in `clusterconfig_types.go`.
- [x] `make go-generate` → deepcopy + CRDs regenerated; verify `charts/` + `examples/` diff is clean.
- [x] `go test ./api/v1alpha1 -run TestServiceAccountIssuer` + `variables_test.go` (Docker, AWS, Nutanix, EKS) → PASS.

### Task 2: Mutation handler — extraArgs + upstream validation + idempotent conflict detection

- [x] Write failing Ginkgo suite in `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/inject_test.go`:
  - omitted variable / worker template → zero patches (no-op, FR-001/SC-003)
  - `issuer` only, `audiences` (single, multi sorted), `jwksURI`, `maxTokenExpiration`, all four → exact `ConsistOf` `extraArgs`
  - pre-existing conflicting `extraArgs` (mismatched value or unset optional flag) → error with remediation message (FR-006)
  - idempotent second `Mutate` / matching pre-existing `extraArgs` → succeeds without duplicating flags (FR-005)
  - overlapping `authentication.jwt[].issuer.url == serviceAccountIssuer.issuer` → error
  - coexistence with unrelated `extraArgs` (`authentication-config`) → preserved
- [x] Implement `inject.go` using upstream `k8s.io/apiserver/pkg/util/webhook.ValidateWebhookURL`, `k8s.io/apimachinery/pkg/util/validation.IsValidPortNum`, and `github.com/spf13/pflag.NewFlagSet`.
- [x] `go test ./pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer -count=1` → PASS.

### Task 3: Wiring — register handler

- [x] In `pkg/handlers/generic/mutation/handlers.go`, import `serviceaccountissuer` and add `serviceaccountissuer.NewPatch()` to `MetaMutators()` (which executes before `ControlPlaneMetaMutators()`'s `sortextraargs.NewPatch()`).
- [x] `go vet ./pkg/handlers/generic/mutation/...` + `go test ./pkg/handlers/generic/mutation/... -count=1` → PASS.

### Task 4: Docs — customization page

- [x] Create `docs/content/customization/kubeadm/service-account-issuer.md` per FR-010 (Hugo auto-indexes pages under `docs/content/customization/kubeadm/`): Kubernetes version note, provider scoping (kubeadm-only, EKS excluded), configuration table (`issuer`, `audiences`, `jwksURI`, `maxTokenExpiration` `[1h, 2^32s]`), examples, `--service-account-extend-token-expiration` note, accurate day-2 rollout token invalidation warning, conflict/composition notes, and removal section.

### Task 5: Integration/e2e — boot-time Cluster template injection & self-contained OIDC/JWKS token validation

- [x] Extend untagged `test/framework/oidc` (`serviceaccountissuer.go`, `serviceaccountissuer_test.go`) with:
  - `EnableServiceAccountIssuerInClusterTemplate` (sharing YAML node injection with `EnableAuthenticationInClusterTemplate` so both `authentication` and `serviceAccountIssuer` compose in `clusterConfig`)
  - `KubeadmControlPlaneHasAPIServerArg`, `ExpectedServiceAccountIssuerExtraArgs`, and `ServiceAccountIssuerConfigCurrent` predicates
  - `FetchOIDCDiscovery`, `VerifyOIDCDiscovery`, `FetchJWKSKeys`, `VerifyServiceAccountJWT` (stdlib RSA/ECDSA verification against `/openid/v1/jwks`), and `VerifyWorkloadServiceAccountIssuer` (`TokenRequest` minting, `iss`/`aud`/`sub` assertions, `SelfSubjectReview` acceptance for configured audience and HTTP `401` rejection for unconfigured audience, and `maxTokenExpiration` lifetime capping)
  - Unit tests covering AWS/Docker/Nutanix Cluster templates, coexistence with `authentication`, KCP predicates, discovery/JWKS signature verification, audience enforcement, and `maxTokenExpiration` capping (plus envtest integration verification against a real `kube-apiserver`)
- [x] Wire boot-time `EnableServiceAccountIssuerInClusterTemplate` and post-provision `ensureWorkloadClusterServiceAccountIssuer` into `test/e2e/quick_start_test.go` on `topology-cilium-helm-addon` across all providers.

### Task 6: Final verification gate

- [x] `make go-generate` clean (no diff beyond `api/` CRDs/deepcopy); `git status` shows no unintended `charts/`/`examples/` changes.
- [x] `make fmt` + `make lint` + `prek` clean (license headers current year).
- [x] `make test` — green across `root`, `api`, and `common` modules.
- [x] SC-003 spot-check: `GeneratePatches` with no `serviceAccountIssuer` set produces byte-identical output (no version bump).

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
