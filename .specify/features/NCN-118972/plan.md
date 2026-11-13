<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: AuthenticationConfiguration (OIDC/JWT) for API server

**Branch**: `NCN-118972-authentication-config` | **Date**: 2026-11-05 | **Spec**: [./spec.md](./spec.md) | **Jira**: [NCN-118972](https://jira.nutanix.com/browse/NCN-118972)

**Goal**: Allow operators to configure the Kubernetes API server's OIDC/JWT authentication by declaring `clusterConfig.authentication` on `KubeadmClusterConfigSpec`; a new generic mutation handler renders `/etc/kubernetes/authentication.yaml` (`apiserver.config.k8s.io/v1` `AuthenticationConfiguration` with `jwt` authenticators), mounts it, and sets `--authentication-config`. Opt-in/no-op when unset.

**Architecture**: API types in `api/v1alpha1/` mirroring upstream `apiserver.config.k8s.io/v1` JWTAuthenticator shape (owned, not vendored via an import that would pull `k8s.io/kubernetes`). New mutation handler under `pkg/handlers/generic/mutation/kubeadm/authentication/` that follows the existing `admissionconfiguration.AddPlugin` pattern (Files + ExtraVolumes + ExtraArgs), registered in `pkg/handlers/generic/mutation/handlers.go` `MetaMutators()`. CEL markers + OpenAPI for early validation. Deterministic YAML via `sigs.k8s.io/yaml` marshaling of a local struct; `jwt` order preserved.

**Tech Stack**: Go; kubebuilder/controller-gen markers + CEL (`XValidation`); CAPI topology mutation (`common/pkg/capi/clustertopology/{handlers/mutation,patches,selectors}` + `variables`); `sigs.k8s.io/yaml`; `testify` for pure API tests, `Ginkgo/Gomega` for handler tests; `make generate` for CRDs/deepcopy.

---

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | Types live in `api/v1alpha1/` only; no business logic in the API module. |
| II. Handler-per-Provider | Pass | Handler lives in `generic` (provider-agnostic), no cross-provider imports; shared helper stays reusable. |
| III. Library-First | Pass | No `cmd/` changes; wiring only in library handler registration. |
| IV. Tests Required | Pass | API validation tests + handler `Mutate` Ginkgo suite (opt-in, multi-issuer, coexistence, no-op, legacy-flag conflict, removal). |
| V. Code Style | Pass | Import aliases per repo; `make fmt`/`make lint` gating. |
| VI. Dependency Management | Pass | No new direct deps; types are owned (not importing `k8s.io/kubernetes/staging/...` which is a depguard/lint concern). |
| VII. Handler Version Safety | Pass | Opt-in: `GeneratePatches` output for inputs not setting `authentication` must be byte-identical; no handler version bump needed (verify via the "omitted variable" test case in existing suite). |
| VIII. Handler Documentation | Pass | New doc `docs/content/customization/kubeadm/authentication.md` + index update in same PR. |

## File Structure

```text
api/v1alpha1/
├── authentication_types.go             # CREATE: types mirroring apiserver.config.k8s.io/v1 JWT Auth (top-level Authentication + JWTAuthenticator etc.)
├── authentication_types_test.go        # CREATE: IsEmpty/validation sanity tests (testify)
├── clusterconfig_types.go              # MODIFY: add Authentication *Authentication `json:"authentication,omitempty"` to KubeadmClusterConfigSpec
├── zz_generated.deepcopy.go            # GENERATED
└── crds/caren.nutanix.com_kubeadmclusterconfigs.yaml  # GENERATED

common/pkg/capi/clustertopology/variables/
└── variables_test.go                   # MODIFY if needed for new variable schema snapshot (only if repo has one)

pkg/handlers/generic/mutation/kubeadm/
├── admissionconfiguration/
│   └── admission_configuration.go      # READ-ONLY reference (reuse pattern, not modify)
└── authentication/
    ├── inject.go                       # CREATE: handler (NewPatch, Mutate, helpers)
    ├── inject_test.go                  # CREATE: Ginkgo suite covering FR-004/005/006 + no-op
    └── doc.go                          # CREATE

pkg/handlers/generic/mutation/handlers.go  # MODIFY: register new patch in MetaMutators()

docs/content/customization/kubeadm/
├── authentication.md                   # CREATE
└── _index.md                           # MODIFY: link new page

hack/examples/...                        # NO CHANGE (feature is opt-in; examples.sync stays green)
```

**Structure Decision**: Single new handler package (mirrors `podsecurityadmission`, `encryptionatrest`) rather than extending `generic` handler. Keeps provider-agnostic, testable in isolation, and avoids coupling authenticator logic into admission helpers.

## Key design facts (read before coding)

- The upstream `AuthenticationConfiguration` Go type lives at `k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/apis/apiserver/v1` — importing that pulls `k8s.io/kubernetes` and is blocked by intent. **Own** a minimal rendering struct in the handler (not exported) whose JSON tags match the upstream shape; marshal to YAML with `sigs.k8s.io/yaml`. The API types in `api/v1alpha1/` are the user-facing source of truth (OpenAPI + CEL) and the handler maps from CRD variable → render struct.
- Control plane selector: `selectors.ControlPlane()` (as used by `podsecurityadmission`, `admissionconfiguration`) — this is a **control plane only** mutation (`KubeadmControlPlaneTemplate`); no worker template patch.
- File helpers: `bootstrapv1.File{Path, Content, Permissions}` and `bootstrapv1.HostPathMount{Name, HostPath, MountPath, ReadOnly}` plus `bootstrapv1.Arg{Name, Value *string}` (per `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration` and `encryptionatrest`). Well-known path: `/etc/kubernetes/authentication.yaml`, volume name `authentication-config`, ExtraArgs flag `authentication-config`.
- CEL patterns in this repo: see `api/v1alpha1/kubelet_types.go` — markers like `// +kubebuilder:validation:XValidation:rule="...",message="..."` alongside field markers. `api/v1alpha1/pod_security_admission_types.go` and `api/v1alpha1/clusterconfig_types.go` for `KubeadmClusterConfigSpec` precedent.
- Determinism: `jwt` order is user intent (do not sort); `audiences` per entry is user intent (do not sort); `claimValidationRules` and `userValidationRules` preserve insertion order.
- No `make examples.sync` change: the default ClusterClasses carry no authentication setting (opt-in). `git status` of `charts/` + `examples/` should be clean aside from the doc page.

## Tasks

Atomic, TDD-ordered. All commands through devbox (`devbox run -- ...`), executed from the worktree root (`.worktrees/NCN-118972-authentication-config`). Use `hack/license-header.go.txt` header with current calendar year.

### Task 1: API types — `Authentication` with JWT authenticator surface

**Files:**

- Create: `api/v1alpha1/authentication_types.go`
- Modify: `api/v1alpha1/clusterconfig_types.go` (add field to `KubeadmClusterConfigSpec`)
- Tests: `api/v1alpha1/authentication_types_test.go`

- [ ] **Step 1: Write the failing test** — in `api/v1alpha1/authentication_types_test.go`, assert that a zero `Authentication` reports empty and that required-field CEL markers will later be enforced (smoke on marshal round-trip). Test should fail to compile because types do not exist.

- [ ] **Step 2: Run to fail** — `devbox run -- go test ./api/v1alpha1/ -run TestAuthentication` → FAIL (undefined types).

- [ ] **Step 3: Create the types** — in `api/v1alpha1/authentication_types.go`:

  ```go
  // Issuer — url, discoveryURL, certificateAuthority, audiences, audienceMatchPolicy
  // AudienceMatchPolicy enum: "" | "MatchAny"
  // JWTAuthenticator — Issuer + ClaimValidationRules + ClaimMappings + UserValidationRules
  // ClaimMappings — Username (PrefixedClaimOrExpression, required), Groups, UID, Extra[]
  // Supporting: ClaimOrExpression, PrefixedClaimOrExpression, ExtraMapping, ClaimValidationRule, UserValidationRule, Authentication { JWT []JWTAuthenticator }
  ```

  Kubebuilder markers:
  - `Authentication.JWT`: `+kubebuilder:validation:Required` + `MinItems=1` + `MaxItems=32` (align with Addons precedent; TODO verify upstream allows up to ~64; pick conservative).
  - `Issuer.URL`: `+kubebuilder:validation:Required` + `Pattern=^https://` + `MinLength=1` + `MaxLength=2048`; add top-level CEL on `Authentication`: CEL to enforce `jwt` issuer URLs unique (`self.jwt.all(...)`).
  - `Issuer.Audiences`: `Required` + `MinItems=1` + `MaxItems=32` + items `MinLength=1` + `MaxLength=256` + `UniqueItems=true`.
  - `ClaimValidationRule`: CEL `XValidation` per entry: exactly one of (`claim` set) xor (`expression` set); when `claim` set `requiredValue` required; `expression` when set needs non-empty; `Message` optional. Use `(has(self.claim) && has(self.requiredValue) && !has(self.expression)) || (!has(self.claim) && has(self.expression))` pattern (see `kubelet_types.go` mutual-exclusivity examples).
  - `ClaimMappings.Username`: required; `PrefixedClaimOrExpression` with `claim` or `expression` (not both); if `claim` present, `prefix` nil allowed (nil → upstream default `<issuer.url>#`, `"-"` → no prefix).
  - Lists capped with `MaxItems` (e.g., `claimValidationRules` 32, `userValidationRules` 32, `extra` 32).

- [ ] **Step 4: Plumb into `KubeadmClusterConfigSpec`** — in `api/v1alpha1/clusterconfig_types.go`:

  ```go
  // Authentication configures API server JWT/OIDC authentication via
  // Structured Authentication Configuration. When unset no authentication
  // file is rendered (no-op). Mutually exclusive with legacy --oidc-* flags.
  // +kubebuilder:validation:Optional
  Authentication *Authentication `json:"authentication,omitempty"`
  ```

- [ ] **Step 5: Pass** — `devbox run -- go test ./api/v1alpha1/ -run TestAuthentication` → PASS. `devbox run -- make generate` → deepcopy + CRDs regenerated; verify `charts/` + `examples/` unchanged. Stage only intended files and leave `charts/`/`examples/` clean. Commit.

---

### Task 2: Mutation handler — file + volume + flag + conflict detection + removal

**Files:**

- Create: `pkg/handlers/generic/mutation/kubeadm/authentication/{doc.go,inject.go,inject_test.go}`

- [ ] **Step 1: Write failing handler tests** — `inject_test.go` Ginkgo suite with at least:
  - `omitted variable → zero patches` (no-op, FR-001/FR-002 in spec context).
  - `one issuer → file added with correct YAML + ExtraVolumes + --authentication-config` (happy path).
  - `two issuers → order preserved in YAML`.
  - `nil/empty JWT → no-op (no patches)` — consistent with other handlers.
  - `both legacy oidc flag and new config present → error`.
  - `coexistence with admissionconfiguration — neither clobbers the other`.
  Use `common/pkg/testutils/capitest` builders and `capitest.JSONPatchMatcher` matchers as seen in `encryptionatrest/inject_test.go` and `podsecurityadmission/inject_test.go`. Run `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/authentication/ -count=1` → FAIL (undefined handler).

- [ ] **Step 2: Implement `inject.go`**:

  ```go
  package authentication
  const (
      VariableName = "authentication"
      AuthenticationConfigPath = "/etc/kubernetes/authentication.yaml"
      AuthenticationConfigArg  = "authentication-config"
  )
  type authPatchHandler struct { variableName, variableFieldPath string }
  func NewPatch() *authPatchHandler { return newHandler(v1alpha1.ClusterConfigVariableName, VariableName) }
  ```

  `Mutate(ctx, obj, vars, holderRef, clusterKey, clusterGetter) error`:
  - `variables.Get[*v1alpha1.Authentication](vars, h.variableName, h.variableFieldPath...)` — `IsNotFoundError` → no-op.
  - If nil or `JWT` empty → **no-op** — return nil immediately, consistent with other handlers (e.g. `podSecurityAdmission` returning nil for nil variable). No active deletion; absence of the variable means no patches.
  - Otherwise: read `obj.Spec.Template.Spec.KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` — if any `Name` startsWith `oidc-` → return error `use of --oidc-* flags is mutually exclusive with authentication; remove the --oidc-* extraArgs`.
  - Else: conflict check if any existing `ExtraArgs` entry has `Name==authentication-config` with a value != well-known path → error.
  - Render: map API types → local render struct (`authnConfig{APIVersion, Kind, JWT []jwtAuth}` with json tags matching upstream), marshal with `sigs.k8s.io/yaml.Marshal`, ensure `spec.Files` upsert on Path, ensure `ExtraVolumes` upsert, ensure `ExtraArgs` upsert.

  Helpers: `ensureFile`, `ensureVolumeMount`, `ensureArg`, `removeFile`, `removeVolumeMount`, `removeArg`, local render types with json tags identical to upstream.

  Confirm: handler is **not** registered yet (registration is Task 3) so the `omitted variable` case is the only thing that matters for the suite in isolation.

- [ ] **Step 3: Pass** — `devbox run -- go test ./pkg/handlers/generic/mutation/kubeadm/authentication/ -count=1` → PASS. Add license headers with current year. Commit.

---

### Task 3: Wiring — register handler in generic mutation pipeline

**Files:**

- Modify: `pkg/handlers/generic/mutation/handlers.go`

- [ ] **Step 1: Register**

  ```go
  import "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
  ```

  Add `authentication.NewPatch()` to `MetaMutators(mgr manager.Manager) []mutation.MetaMutator` return slice (control-plane generic mutators pipeline). Placement near `podsecurityadmission.NewPatch()`.

- [ ] **Step 2: Verify** — `devbox run -- go vet ./pkg/handlers/generic/mutation/...` and `devbox run -- go test ./pkg/handlers/generic/mutation/... -count=1` (smoke). Also verify the no-op test still shows byte-identical output for omitted-variable case. `make generate` should not touch `charts/`/`examples/`. Commit.

---

### Task 4: Docs — customization page + index

**Files:**

- Create: `docs/content/customization/kubeadm/authentication.md`
- Modify: `docs/content/customization/kubeadm/_index.md` (or `docs/content/customization/_index.md` if the index lives one level up — verify on branch)
- (If needed) update `docs/content/getting-started/...` cross-link

- [ ] **Step 1: Write `authentication.md`** — include: what it does (OIDC/JWT via `AuthenticationConfiguration`), minimum Kubernetes version (1.29 beta, recommended 1.31+), mutual exclusivity note vs `--oidc-*`, configuration table per field group (Issuer, ClaimValidationRules, ClaimMappings.*), single-issuer example, multi-issuer example, CEL expression example, and the canonical YAML snippet. Use the same front-matter style as `pod-security-admission.md`.

- [ ] **Step 2: Update index** — add link to `authentication.md` in the kubeadm customization index.

- [ ] **Step 3: Preview** — `devbox run -- make preview-docs` (or `hugo server` per `AGENTS.md`) — page renders without warnings. Commit.

---

### Task 5: Final verification gate

- [ ] `devbox run -- make generate` — clean (no diff beyond `api/` CRDs/deepcopy). `git status` shows no unintended `charts/`/`examples/` changes.
- [ ] `devbox run -- make lint` and `devbox run -- make fmt` — clean.
- [ ] `devbox run -- make test` (or at least `make test.root` + `go test ./api/v1alpha1` + `go test ./pkg/handlers/generic/mutation/kubeadm/authentication`) — green.
- [ ] Manual spec acceptance spot-checks (TDD cases above) still green; no version bump required for existing clusters.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Importing `k8s.io/kubernetes` staging types pulls forbidden deps | Own the render types; mirror upstream JSON shape via local structs with identical tags. |
| CEL `XValidation` for "exactly one of claim/expression" is tricky | Copy the pattern from `api/v1alpha1/kubelet_types.go`; add API-level integration test for rejection cases. |
| Handler must coexist with `admissionconfiguration` handler | Use distinct well-known path/name/flag; tests cover additive composition. |

## Out of Scope (explicitly not in this plan)

- `--oidc-*`flag migration assistant beyond the handler error.
- `anonymous` / `webhook` subsections of `AuthenticationConfiguration` or `--service-account-*` flags.
- E2E against a real cluster (covered by `test/e2e` label-driven tests if desired later).
