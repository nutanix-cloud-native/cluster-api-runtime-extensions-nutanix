<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: AuthenticationConfiguration (OIDC/JWT) for API server

**Branch**: `NCN-118972-authentication-config` | **Date**: 2026-11-05 | **Spec**: [./spec.md](./spec.md) | **Jira**: [NCN-118972](https://jira.nutanix.com/browse/NCN-118972)

**Goal**: Allow operators to configure the Kubernetes API server's OIDC/JWT authentication by declaring `clusterConfig.authentication` on `KubeadmClusterConfigSpec`; a new generic mutation handler renders `/etc/kubernetes/authentication.yaml` (`apiserver.config.k8s.io/v1` `AuthenticationConfiguration` with `jwt` authenticators), mounts it, and sets `--authentication-config`. Opt-in/no-op when unset.

**Architecture**: API types in `api/v1alpha1/` mirroring upstream `apiserver.config.k8s.io/v1` JWTAuthenticator shape. New mutation handler under `pkg/handlers/generic/mutation/kubeadm/authentication/` that renders `k8s.io/apiserver/pkg/apis/apiserver/v1.AuthenticationConfiguration`, validates it with `k8s.io/apiserver/pkg/apis/apiserver/validation.ValidateAuthenticationConfiguration`, and configures `Files` + `ExtraVolumes` + `ExtraArgs`, registered in `pkg/handlers/generic/mutation/handlers.go` `MetaMutators()`. CEL markers + OpenAPI + upstream apiserver validation for early rejection. Deterministic YAML via `sigs.k8s.io/yaml`; `jwt` order preserved.

**Tech Stack**: Go; kubebuilder/controller-gen markers + CEL (`XValidation` with `exists_one`); CAPI topology mutation (`common/pkg/capi/clustertopology/{handlers/mutation,patches,selectors}` + `variables`); `k8s.io/apiserver` (`v1` + `validation` + `authentication/cel`); `sigs.k8s.io/yaml`; `testify` for unit tests, `Ginkgo/Gomega` for handler + e2e tests; stdlib `crypto/rsa` + `net/netip` mock OIDC in untagged `test/framework/oidc` (zero new `go.mod` dependencies).

---

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | Types live in `api/v1alpha1/` only; no business logic in the API module. |
| II. Handler-per-Provider | Pass | Handler lives in `generic` (provider-agnostic), no cross-provider imports; shared helper stays reusable. |
| III. Library-First | Pass | No `cmd/` changes; wiring only in library handler registration. |
| IV. Tests Required | Pass | API validation tests + handler `Mutate`/`Render` tests + untagged `test/framework/oidc` unit tests + provider-agnostic e2e with in-cluster mock OIDC token validation. |
| V. Code Style | Pass | Import aliases per repo (`clusterv1` for `sigs.k8s.io/cluster-api/api/core/v1beta2`); `make fmt`/`make lint` gating. |
| VI. Dependency Management | Pass | Zero new direct dependencies in `go.mod`; `k8s.io/apiserver` is already present in `go.mod` and JWT signing uses stdlib `crypto/rsa`. |
| VII. Handler Version Safety | Pass | Opt-in: `GeneratePatches` output for inputs not setting `authentication` must be byte-identical; no handler version bump needed (verify via the "omitted variable" test case in existing suite). |
| VIII. Handler Documentation | Pass | New doc `docs/content/customization/kubeadm/authentication.md` in same PR (Docsy auto-indexes child pages). |

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

pkg/webhook/cluster/
├── authentication_validator.go         # CREATE: fail-fast Cluster admission validation
├── authentication_validator_test.go    # CREATE: unit + envtest webhook validation tests
└── validator.go                        # MODIFY: wire NewAuthenticationValidator into MultiValidatingHandler

docs/content/customization/kubeadm/
└── authentication.md                   # CREATE (Docsy auto-indexes child pages)

test/framework/oidc/
├── oidc.go                             # CREATE: untagged mock OIDC bundle/deploy, Service CIDR static IP, stdlib RS256 JWT mint, authn/KCP predicates
└── oidc_test.go                        # CREATE: unit tests run under `make test`

test/e2e/
├── config/caren.yaml                   # MODIFY: 20m cluster/control-plane/worker/node readiness intervals
└── quick_start_test.go                 # MODIFY: inject boot-time authentication with pre-generated CA + static Service ClusterIP on `topology-cilium-helm-addon`, deploy mock OIDC in PostMachinesProvisioned, validate username/groups/extra + invalid JWTs

hack/examples/...                        # NO CHANGE (feature is opt-in; examples.sync stays green)
```

**Structure Decision**: Single new handler package (mirrors `podsecurityadmission`, `encryptionatrest`) rather than extending `generic` handler. Keeps provider-agnostic, testable in isolation, and avoids coupling authenticator logic into admission helpers.

## Key design facts (read before coding)

- `k8s.io/apiserver` is already in root `go.mod` (`k8s.io/kubernetes` is what `depguard` blocks, not `k8s.io/apiserver`). Render via `k8s.io/apiserver/pkg/apis/apiserver/v1.AuthenticationConfiguration` and validate with `k8s.io/apiserver/pkg/apis/apiserver/validation.ValidateAuthenticationConfiguration` + `k8s.io/apiserver/pkg/authentication/cel.NewDefaultCompiler()` so invalid CEL expressions, reserved `k8s.io`/`kubernetes.io` extra keys, invalid URLs, or malformed CA PEMs fail during topology mutation rather than bricking `kube-apiserver`.
- Control plane selector: `selectors.ControlPlane()` (as used by `podsecurityadmission`, `admissionconfiguration`) — this is a **control plane only** mutation (`KubeadmControlPlaneTemplate`); no worker template patch.
- File helpers: `bootstrapv1.File{Path, Content, Permissions}` and `bootstrapv1.HostPathMount{Name, HostPath, MountPath, ReadOnly}` plus `bootstrapv1.Arg{Name, Value *string}`. Expected path: `/etc/kubernetes/authentication.yaml`, volume name `authentication-config`, ExtraArgs flag `authentication-config`. Reject nil/conflicting `--authentication-config` args and conflicting `ExtraVolumes` names/paths.
- CEL patterns in this repo: use `self.jwt.all(a, self.jwt.exists_one(b, b.issuer.url == a.issuer.url))` for uniqueness (handles identical duplicate entries with zero intermediate list allocation).
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
  - `ClaimMappings.Username`: required; `PrefixedClaimOrExpression` with `expression`, or `claim` plus `prefix` (not both). `prefix` is required when `claim` is set; `""` disables prefixing. `"-"` is a literal prefix. kube-apiserver rejects a missing prefix.
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

### Task 4: Docs — customization page

**Files:**

- Create: `docs/content/customization/kubeadm/authentication.md`

- [ ] **Step 1: Write `authentication.md`** — include: what it does (OIDC/JWT via `AuthenticationConfiguration`), minimum Kubernetes version (`>= 1.34`), mutual exclusivity note vs `--oidc-*`, single-issuer example, multi-issuer example, CEL expression example, and the matching rendered `AuthenticationConfiguration` YAML snippet. Use the same front-matter style as `pod-security-admission.md`.

- [ ] **Step 2: Preview** — `make preview-docs` (or `hugo` build) — page renders without warnings. Commit.

---

### Task 5: Integration/e2e — boot-time static `ClusterIP` mock OIDC token validation (provider-agnostic, no external service)

**Files:**

- Create: `test/framework/oidc/{oidc.go,oidc_test.go}` (untagged: pre-generated RSA 2048 + self-signed CA + server cert with IP SAN, Service CIDR static IP selection, YAML template injection, JWKS/discovery ConfigMap + Service/Deployment apply, stdlib RS256 `MintJWT`, authn/KCP predicates, `SelfSubjectReview` UserInfo assertions)
- Modify: `test/e2e/config/caren.yaml` (`20m` provisioning/readiness intervals)
- Modify: `test/e2e/quick_start_test.go` (in `BeforeEach` for `topology-cilium-helm-addon`, resolve Service CIDR from template, pre-generate mock OIDC bundle for static `ClusterIP`, and inject `clusterConfig.authentication` into the Cluster template; in `PostMachinesProvisioned`, verify live Service CIDR, deploy mock OIDC on static `ClusterIP`, and validate `username`/`groups`/`extra` + invalid JWT rejection)

- [ ] **Step 1: Untagged helper + unit tests** — `test/framework/oidc`: `NewBundle(clusterIP)` generates CA + server cert (IP SAN in `IPAddresses`, DNS names in `DNSNames`) + RSA signing key; `StaticServiceIPFromCIDR` picks offset `+12` inside Kubernetes's KEP-3070 static Service IP band; `EnableAuthenticationInClusterTemplate` resolves `${SERVICE_CIDR}` and injects `clusterConfig.authentication` at YAML node level.

- [ ] **Step 2: Spec** — `quick_start_test.go` `topology-cilium-helm-addon`: workload Cluster boots once with `clusterConfig.authentication` already rendered; `PostMachinesProvisioned` verifies live Service CIDR and `KubeadmControlPlane` authentication file, deploys `mock-oidc` with `Service.spec.clusterIP`, waits for `kube-apiserver` asynchronous OIDC discovery via `SelfSubjectReview` (checking `username`, `groups`, `extra`), and verifies invalid JWTs (`401`) and admin kubeconfig access.

- [ ] **Step 3: Verify** — `make test` + `make lint` + `make pre-commit` green; e2e runs under `--focus="Quick start"` on any provider.

---

### Task 6: Final verification gate

- [ ] `make go-generate` — clean (no diff beyond `api/` CRDs/deepcopy). `git status` shows no unintended `charts/`/`examples/` changes.
- [ ] `make lint` and `make fmt` — clean.
- [ ] `make test` — green across all Go modules (`root`, `api`, `common`, etc.).
- [ ] `make pre-commit` — green.

---

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Invalid CEL, URL, PEM, or reserved extra key bricks `kube-apiserver` | Validate rendered config in handler via `ValidateAuthenticationConfiguration` + `authenticationcel.NewDefaultCompiler()` alongside CRD OpenAPI/CEL. |
| CEL uniqueness check with `b != a` misses identical entries | Use `self.jwt.all(a, self.jwt.exists_one(b, b.issuer.url == a.issuer.url))` (and `exists_one` on `extra[].key`). |
| Handler must coexist with `admissionconfiguration` handler | Use distinct path/volume/flag and reject conflicting volume names/paths or `--authentication-config` values. |
| `hostNetwork` `kube-apiserver` cannot resolve CoreDNS, and day-2 KCP rollout is slow on Nutanix | Pre-generate CA + server cert for a static IP in the cluster's Service CIDR (`offset +12`, inside Kubernetes's static `ClusterIP` band) and configure `clusterConfig.authentication` at Cluster creation time; `kube-apiserver` polls OIDC discovery asynchronously every 10s until `mock-oidc` is `Available`. |

## Out of Scope (explicitly not in this plan)

- `--oidc-*`flag migration assistant beyond the handler error.
- `anonymous` / `webhook` subsections of `AuthenticationConfiguration` or `--service-account-*` flags.
- External IdP (Okta/Auth0) — covered by in-cluster mock OIDC.
