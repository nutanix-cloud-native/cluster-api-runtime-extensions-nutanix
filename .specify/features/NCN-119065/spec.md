<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Feature Specification: Service Account Issuer Configuration for API Server

**Jira Ticket**: [NCN-119065](https://jira.nutanix.com/browse/NCN-119065)
**Feature Branch**: `NCN-119065-service-account-issuer`
**Created**: 2026-11-10
**Status**: Draft
**Input**: User description: "add service account issuer configuration to api server"

## Background

Kubernetes ServiceAccount tokens are JWTs issued by the API server. The issuer identity (`iss` claim) and audience validation are controlled by API server flags:

- `--service-account-issuer` — identifier asserted in the `iss` claim; must be an `https://` URL for OIDC discovery (`/.well-known/openid-configuration` and `openid/v1/jwks`) to be served.
- `--api-audiences` — identifiers of the API that tokens must be bound to; defaults to the issuer URL when `service-account-issuer` is set.
- `--service-account-jwks-uri` — optional override for the JWKS URI advertised in the discovery document.
- `--service-account-max-token-expiration` — maximum lifetime for ServiceAccount tokens (Go duration, e.g. `24h`).
- `--service-account-key-file`, `--service-account-signing-key-file`, `--service-account-extend-token-expiration`, etc. — key and lifetime controls.

CAREN today has no way to configure the issuer. For kubeadm-based providers (AWS, Docker, Nutanix) the API server runs with kubeadm defaults (`https://kubernetes.default.svc.cluster.local`). Operators needing workload identity (IRSA, Azure Workload Identity, Nutanix IAM, custom OIDC federation) must hand-edit `KubeadmControlPlaneTemplate` or front the cluster with an external issuer proxy.

Existing CAREN API server configuration surfaces (`podSecurityAdmission`, `encryptionAtRest`, `extraAPIServerCertSANs`) follow an opt-in variable on `KubeadmClusterConfigSpec` + provider-agnostic mutation handler that patches `KubeadmControlPlaneTemplate.spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs`. This feature follows the same pattern.

This feature concerns **ServiceAccount token issuer** configuration only. It does **not** configure OIDC/JWT authentication of end-users (`--oidc-*` / `--authentication-config`), which is covered by the separate AuthenticationConfiguration feature (NCN-118972).

## User Scenarios & Testing

### User Story 1 - Operator configures ServiceAccount issuer for workload identity (Priority: P1)

An operator wants pods to obtain ServiceAccount tokens whose `iss` is an externally-reachable OIDC URL (e.g., `https://oidc.example.com/id/ABC123`) so that an external IAM system (AWS IAM, Azure Workload Identity, GCP Workload Identity, Vault, Nutanix IAM) can federate and grant cloud credentials via projected ServiceAccount tokens.

**Why this priority**: Core capability requested. Without it, workload identity requires out-of-band mutation of the control plane.

**Independent Test**: Apply a Cluster whose `clusterConfig.serviceAccountIssuer` sets `issuer: https://oidc.example.com` (or with additional audiences). Verify the generated `KubeadmControlPlaneTemplate` contains `apiServer.extraArgs` with `service-account-issuer=https://oidc.example.com` (and `api-audiences` if configured). Verify the workload cluster's `/.well-known/openid-configuration` serves that issuer and that a projected token's `iss` matches.

**Acceptance Scenarios**:

1. **Given** a Cluster with `clusterConfig.serviceAccountIssuer.issuer: https://oidc.example.com`, **When** the topology mutation handler runs, **Then** it adds `apiServer.extraArgs` entry `service-account-issuer=https://oidc.example.com` to the `KubeadmControlPlaneTemplate`.
2. **Given** a Cluster with `issuer: https://oidc.example.com` and `audiences: ["sts.amazonaws.com", "k8s.example.com"]` (any order), **When** handled, **Then** it adds both `service-account-issuer=https://oidc.example.com` and `api-audiences=k8s.example.com,sts.amazonaws.com` (sorted).
3. **Given** a Cluster with `issuer: https://oidc.example.com` and `jwksURI: https://oidc.example.com/.well-known/jwks.json`, **When** handled, **Then** it adds `service-account-jwks-uri` with that value in addition to the issuer.

---

### User Story 2 - Operator upgrades CAREN without triggering rollout (Priority: P1)

An operator upgrades CAREN on a management cluster with many workload clusters that do **not** set the new field. No workload cluster rolls.

**Why this priority**: Constitution principle VII (handler version safety). The feature must be opt-in and invisible when unset.

**Independent Test**: For a representative Cluster that does not set `serviceAccountIssuer`, compare `GeneratePatches` output of the new handler (and overall control-plane pipeline) before and after the change. Output must be byte-identical. Confirm no handler version bump is introduced solely by this change.

**Acceptance Scenarios**:

1. **Given** an existing Cluster without `serviceAccountIssuer` set, **When** CAREN is upgraded to include this feature, **Then** neither its `KubeadmControlPlaneTemplate` nor its `ClusterClass` patches change, and no Machine rollout is triggered.
2. **Given** the PR diff, **When** reviewing, **Then** no existing handler's version name is bumped solely due to this work.

---

### User Story 3 - Invalid issuer configuration is rejected early (Priority: P2)

An operator typos an issuer URL or omits required fields. They get a clear error at Cluster admission time rather than a broken API server after rollout.

**Why this priority**: Prevents a broken API server (invalid flag would prevent API server start). Early rejection matches existing CAREN validation patterns (`PodSecurityAdmission`, `Etcd`, `EncryptionAtRest`).

**Independent Test**: Apply Cluster YAML that violates each required-field / format constraint and assert admission is denied with a message naming the field.

**Acceptance Scenarios**:

1. **Given** a Cluster YAML whose `serviceAccountIssuer.issuer` is empty or not an `https://` URL (e.g., `http://`, `ftp://`, or bare string), **When** applied, **Then** admission is rejected.
2. **Given** a Cluster YAML whose `serviceAccountIssuer.issuer` exceeds allowed length or contains invalid characters, **When** applied, **Then** admission is rejected.
3. **Given** a valid Cluster that sets the field, **When** patched to remove `issuer` while keeping the object, **Then** the update is rejected (issuer is required when the block is present).

---

### User Story 4 - Operator configures issuer with custom audiences for cross-service validation (Priority: P2)

An operator needs tokens to be valid for multiple audiences (e.g., Kubernetes API + external service) so that the same projected token can be validated by both the API server and an external system that checks `aud`.

**Why this priority**: Audiences are the second most common customization after issuer; supports multi-audience workload identity patterns without extra work.

**Independent Test**: Apply Cluster with `audiences: ["api", "sts.amazonaws.com"]` and verify `api-audiences` is set as comma-joined value and that token `aud` contains both entries.

**Acceptance Scenarios**:

1. **Given** an issuer with `audiences: ["k8s.example.com"]`, **When** rendered, **Then** `api-audiences` equals that single value.
2. **Given** an issuer with `audiences: ["a", "b", "c"]`, **When** rendered, **Then** `api-audiences` equals `a,b,c` (already sorted; any permutation sorts to same).
3. **Given** an issuer with `audiences: ["sts.amazonaws.com", "k8s.example.com"]`, **When** rendered, **Then** `api-audiences` equals `k8s.example.com,sts.amazonaws.com` (alphabetically sorted regardless of input order).
4. **Given** an issuer without `audiences`, **When** rendered, **Then** no `api-audiences` flag is set (API server defaults to issuer URL).

### User Story 5 - Operator configures maximum token lifetime (Priority: P2)

An operator wants to bound explicit ServiceAccount token lifetimes (e.g., `24h`, within `kube-apiserver`'s `[1h, 2^32s]` range) for security compliance, so long-lived `TokenRequest` or explicit projected volume tokens are capped by the API server. Note that `kube-apiserver`'s `--service-account-extend-token-expiration` defaults to `true`, which extends default admission-injected Pod projected tokens (`3607s`) up to 1 year unless pods specify `expirationSeconds` explicitly.

**Why this priority**: Common hardening need; complements issuer/audience configuration without requiring key management.

**Independent Test**: Apply Cluster with `serviceAccountIssuer.issuer: https://oidc.example.com` and `maxTokenExpiration: 24h`. Verify the generated `KubeadmControlPlaneTemplate` contains `service-account-max-token-expiration=24h0m0s` in `apiServer.extraArgs`. Request a token with `expirationSeconds` exceeding the max and verify it is capped by the API server.

**Acceptance Scenarios**:

1. **Given** an issuer with `maxTokenExpiration: 1h`, **When** rendered, **Then** `service-account-max-token-expiration` equals `1h0m0s`.
2. **Given** an issuer without `maxTokenExpiration`, **When** rendered, **Then** no `service-account-max-token-expiration` flag is set (API server uses default).
3. **Given** an invalid duration (e.g., `not-a-duration`, `0s`, `59m59s` below `1h`, or `> 1193046h28m16s`), **When** applied, **Then** admission is rejected with a message naming the field.

---

### User Story 6 - Provider-agnostic e2e validation without external services (Priority: P2)

An operator/CI wants confidence that ServiceAccount issuer configuration works on any kubeadm provider (AWS, Docker, Nutanix) without needing a public OIDC provider, cloud IAM, or internet access.

**Why this priority**: Prevents drift; ensures the mutation handler and OIDC discovery compose correctly across providers in CI (especially Docker which needs only a container runtime).

**Independent Test**: Run a single e2e spec that injects `clusterConfig.serviceAccountIssuer` into the `topology-cilium-helm-addon` Cluster template before provisioning on each provider (alongside `clusterConfig.authentication`, avoiding a day-2 control-plane rollout and proving additive composition), using synthetic HTTPS URLs (`issuer`, unsorted `audiences`, `jwksURI`, and `maxTokenExpiration`). Verify the reconciled `KubeadmControlPlane` has the expected `extraArgs` (`service-account-issuer`, alphabetically sorted `api-audiences`, `service-account-jwks-uri`, `service-account-max-token-expiration`). After the workload cluster is Ready, via its kubeconfig verify `/.well-known/openid-configuration` returns the configured `issuer` and `jwks_uri` override, fetch `/openid/v1/jwks` to cryptographically verify minted ServiceAccount JWT signatures in-process, verify tokens bound to a configured audience authenticate via `SelfSubjectReview` while tokens bound only to an unconfigured audience are rejected (`401`), and verify `maxTokenExpiration` caps requested token lifetimes.

**Acceptance Scenarios**:

1. **Given** a Cluster per provider with synthetic `https://` issuer, audiences, `jwksURI`, and `maxTokenExpiration` injected at Cluster creation time, **When** reconciled, **Then** `KubeadmControlPlane` contains `service-account-issuer`, sorted `api-audiences`, `service-account-jwks-uri`, and `service-account-max-token-expiration` (coexisting with `authentication-config`), and sibling flavors omitting the field remain unaffected.
2. **Given** the workload cluster is Ready, **When** querying `/.well-known/openid-configuration` and `/openid/v1/jwks` via its kubeconfig and minting `TokenRequest` tokens, **Then** discovery serves the configured `issuer` and `jwks_uri`, JWT signatures verify against `/openid/v1/jwks`, `iss`/`aud`/`sub` match, `SelfSubjectReview` accepts configured audiences and rejects unconfigured audiences with `401`, and `maxTokenExpiration` caps longer `expirationSeconds` requests.
3. **Given** no external/cloud service is available, **When** the suite runs, **Then** it still passes (no internet/IdP required — API server serves discovery and JWKS locally).

---

### Edge Cases

- What happens when the user also sets `service-account-issuer`, `api-audiences`, `service-account-jwks-uri`, or `service-account-max-token-expiration` directly via `KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` (out-of-band)? If an existing `extraArgs` entry sets any of those flags with a conflicting value (or sets an optional flag that `serviceAccountIssuer` leaves unset), the handler MUST return a clear error directing the user to remove the direct extraArgs and use `serviceAccountIssuer` instead. If an existing entry already matches the exact desired value, `Mutate` succeeds idempotently (matching `authentication`).
- What happens when an existing cluster already has `service-account-issuer` set to kubeadm's implicit default `https://kubernetes.default.svc.cluster.local` and the operator sets a new `serviceAccountIssuer.issuer` on day 2? The change triggers a control-plane rollout; because CAREN configures a single `--service-account-issuer` value, previously minted tokens carrying the old `iss` claim (or an audience dropped from `audiences`) are immediately rejected (`401 Unauthorized`) by the rolled API server until pods refresh their tokens / restart. Operators should prefer setting `serviceAccountIssuer` at cluster creation time.
- What happens when the issuer URL points to an external OIDC provider that is not yet reachable at API server start? The API server still starts; token issuance uses that issuer string. OIDC discovery (`/.well-known/openid-configuration`) is served by the API server itself based on the issuer URL, not by dialing the external provider. No handler-level reachability check is needed.
- How does this compose with `AuthenticationConfiguration` (NCN-118972) or `PodSecurityAdmission` or `EncryptionAtRest`? They use different flags/files/paths (`--authentication-config`, `--admission-control-config-file`, `encryption-provider-config`, etc.) and compose additively, except `kube-apiserver` forbids `authentication.jwt[].issuer.url` from overlapping `--service-account-issuer` (`disallowedIssuers`). Both CRD CEL on `KubeadmClusterConfigSpec` and handler validation reject overlapping `authentication.jwt[].issuer.url == serviceAccountIssuer.issuer`.
- Removing the field (setting `serviceAccountIssuer` back to nil/omitted) causes the handler to emit no patches against the `ClusterClass` `KubeadmControlPlaneTemplate`. Clusters that never set `serviceAccountIssuer` are unaffected, while removing it from a cluster that previously enabled it reconciles `KubeadmControlPlane` back to the unpatched `ClusterClass` template — removing the flags and rolling control-plane machines back to kubeadm defaults.
- Key file management (`--service-account-key-file`, `--service-account-signing-key-file`) is out of scope for this feature. kubeadm already manages a signing key pair; custom key rotation is not covered initially.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST add an **opt-in** field `serviceAccountIssuer` to `KubeadmClusterConfigSpec` in `api/v1alpha1/` (alongside `podSecurityAdmission`, `encryptionAtRest`, `etcd`, `dns`, `authentication`). When the field is unset/nil the feature is a no-op; `GeneratePatches` output for such inputs MUST be byte-identical to pre-change output.
- **FR-002**: The `serviceAccountIssuer` type MUST expose `issuer` (required, non-empty, must be an `https://` URL with non-empty hostname, no userinfo, and no query or fragment, max length 2048, matching OpenID Discovery 1.0 Section 3 issuer requirements). It MUST carry kubebuilder + CEL validation (`Required`, `Format=uri`, ``Pattern=`^https://[^/?#@\s]+(/[^\s?#]*)?$` ``, `isURL(self) && url(self).getHostname() != ''`, `MinLength=9`, `MaxLength=2048`) and handler-side validation via upstream `k8s.io/apiserver/pkg/util/webhook.ValidateWebhookURL`.
- **FR-003**: The type MAY expose `audiences` (optional, list of non-empty strings without whitespace, commas, or double quotes ``Pattern=`^[^\s,"]+$` ``, each 1-253 chars, max 50 entries, unique items). When set, the handler MUST set `--api-audiences` to the comma-joined list **sorted alphabetically** (to avoid rollouts on reorder) and verify round-trip parsing via upstream `github.com/spf13/pflag`. When absent/empty, the handler MUST NOT set `--api-audiences` (API server defaults to issuer).
- **FR-004**: The type MAY expose `jwksURI` (optional, when set must be an `https://` URL with non-empty hostname, no userinfo, and no fragment — query parameters allowed per OpenID Discovery 1.0 Section 3 — max length 2048). When set, the handler MUST set `--service-account-jwks-uri` to that value.
- **FR-004b**: The type MAY expose `maxTokenExpiration` (optional, `*metav1.Duration`, bounded to `[1h, 1193046h28m16s]` matching `kube-apiserver`'s `[1h, 2^32s]` startup validation). When set, the handler MUST set `--service-account-max-token-expiration` to `Duration.String()`. When absent, the flag MUST NOT be set.
- **FR-005**: A **new provider-agnostic topology mutation handler** under `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/` MUST, when the variable is present and non-nil, add/update `KubeadmControlPlaneTemplate.spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs` entries for `service-account-issuer` (always) and optionally `api-audiences`, `service-account-jwks-uri`, and `service-account-max-token-expiration`. It MUST be idempotent when invoked on an already-matching template and MUST NOT add any other flags.
- **FR-006**: If `apiServer.extraArgs` already contains `service-account-issuer`, `api-audiences`, `service-account-jwks-uri`, or `service-account-max-token-expiration` with a conflicting value (or for an optional flag not configured in `serviceAccountIssuer`), the handler MUST return an error explaining the conflict and directing the user to remove the direct extraArgs and use `serviceAccountIssuer` instead. It MUST NOT silently overwrite.
- **FR-007**: API types MUST carry kubebuilder / CEL validation: `issuer` required when `serviceAccountIssuer` is present; `issuer` (`https://`, non-empty hostname, no userinfo/query/fragment) and `jwksURI` (`https://`, non-empty hostname, no userinfo/fragment); `audiences` maxItems 50, unique, each item minLength 1, maxLength 253, no whitespace/commas/quotes; `maxTokenExpiration` when set must be in `[1h, 1193046h28m16s]`; `KubeadmClusterConfigSpec` CEL must forbid `serviceAccountIssuer.issuer` overlapping `authentication.jwt[].issuer.url`; the whole block is optional (pointer, nil = absent).
- **FR-008**: The rendered `extraArgs` MUST be deterministic and stable. Ordering is delegated to the existing `sortextraargs` handler (which sorts `apiServer.extraArgs` by name) — the new handler does not need to sort, but MUST be registered **before** `sortextraargs` so final output is stable.
- **FR-009**: The feature MUST be available to **kubeadm-based providers only** (AWS, Docker, Nutanix). EKS is excluded (managed control plane — no access to API server flags). This matches `podSecurityAdmission` and `encryptionAtRest` scoping.
- **FR-010**: The feature MUST be documented under `docs/content/customization/kubeadm/service-account-issuer.md` (Hugo auto-indexes directory pages under `kubeadm/`) with configuration tables, what it does, how to enable it, examples for single issuer and with audiences/JWKS URI and maxTokenExpiration, provider scoping note, Kubernetes version note, `--service-account-extend-token-expiration` behavior note, accurate day-2 rollout token invalidation warning, and removal section.
- **FR-011**: An e2e test MUST exist that validates ServiceAccount issuer configuration on all kubeadm providers without requiring public/external services, matching the self-contained architecture of the authentication config e2e (`test/framework/oidc` + boot-time Cluster template injection in `test/e2e/quick_start_test.go`). It MUST inject a synthetic `https://` `serviceAccountIssuer` (`issuer`, unsorted `audiences`, `jwksURI`, `maxTokenExpiration`) into `topology-cilium-helm-addon` at Cluster creation time (avoiding a day-2 rollout and verifying coexistence with `authentication`), assert the reconciled `KubeadmControlPlane` has the expected `extraArgs`, and after the workload cluster is Ready verify `/.well-known/openid-configuration` (`issuer` and `jwks_uri`), `/openid/v1/jwks` cryptographic signature validation of `TokenRequest` JWTs, `SelfSubjectReview` acceptance/rejection across configured vs unconfigured audiences, and `maxTokenExpiration` lifetime capping. It MUST be runnable via `make e2e-test E2E_LABEL='provider:Docker'` (or AWS/Nutanix) with no cloud IAM/IdP.

### Key Entities

- **KubeadmClusterConfigSpec.serviceAccountIssuer**: Opt-in block on the provider-agnostic KubeadmClusterConfig that carries the desired ServiceAccount token issuer configuration. Lives in `api/v1alpha1/` alongside other API server-related config. Affects `KubeadmControlPlaneTemplate` only (control plane).
- **ServiceAccountIssuer**: One issuer configuration: required `issuer` URL + optional `audiences` list + optional `jwksURI` override + optional `maxTokenExpiration` duration. Maps 1:1 to API server flags `service-account-issuer`, `api-audiences`, `service-account-jwks-uri`, `service-account-max-token-expiration`.
- **ServiceAccount mutation handler**: Provider-agnostic handler that turns the variable into `apiServer.extraArgs` patches on `KubeadmControlPlaneTemplate`. Registered in `pkg/handlers/generic/mutation/handlers.go` `MetaMutators()`.
- **ServiceAccountIssuer e2e spec**: Ginkgo e2e in `test/e2e/` that runs against any kubeadm provider with a synthetic `https://` issuer, no external/cloud dep, asserting template extraArgs and workload-cluster OIDC discovery/projected token.

## Success Criteria

### Measurable Outcomes

- **SC-001**: An operator can add `clusterConfig.serviceAccountIssuer.issuer: https://oidc.example.com` and the resulting `KubeadmControlPlaneTemplate` contains `extraArgs` with `service-account-issuer=https://oidc.example.com` and no other new flags; the patch is applied on next reconciliation and the API server serves discovery at that issuer.
- **SC-002**: With `audiences: ["sts.amazonaws.com", "k8s.example.com"]` (any order) the resulting `api-audiences` equals `k8s.example.com,sts.amazonaws.com` (sorted, stable).
- **SC-003**: 100% of inputs that did **not** set `serviceAccountIssuer` produce byte-identical `GeneratePatches` output to pre-change behavior (no rollout on upgrade).
- **SC-004**: Any Cluster YAML that sets `serviceAccountIssuer` but omits or mis-formats `issuer` (empty, non-https, too long) is rejected at admission with a message naming the offending field.
- **SC-005**: Documentation page is published at `docs/content/customization/kubeadm/service-account-issuer.md` (auto-indexed by Hugo under Kubeadm customizations), with working examples that can be copied directly.
- **SC-006**: A Cluster that sets `service-account-issuer` or `api-audiences` directly via `extraArgs` and also sets `serviceAccountIssuer` receives a clear handler error rather than a silent conflict or broken API server rollout.
- **SC-007**: With `maxTokenExpiration: 1h` the resulting `KubeadmControlPlaneTemplate` contains `service-account-max-token-expiration=1h0m0s`; without it the flag is absent; durations `< 1h` or `> 1193046h28m16s` are rejected at admission.
- **SC-008**: An e2e run with `provider:Docker` (or AWS/Nutanix) and synthetic `https://` issuer passes without external services: template has `service-account-issuer`, workload cluster discovery returns the issuer, all configured audiences authenticate via `SelfSubjectReview`, unconfigured audiences return `401`, and omitting the field causes no rollout.

## Out of Scope

- Managing the IdP / OIDC provider itself, Dex, `kube-oidc-proxy`, or cloud IAM role bindings — this feature only configures the API server's issuer identity, not the consumer side.
- `--service-account-key-file` / `--service-account-signing-key-file` custom key management and rotation — kubeadm already generates and rotates a key pair; custom key injection can be added later if needed without breaking this API (additive optional fields).
- `--service-account-extend-token-expiration` tuning — defaults to `true` in `kube-apiserver`, which extends default admission-injected Pod projected tokens (`3607s`) up to 1 year regardless of `maxTokenExpiration`. Documented explicitly; if needed later, can be added additively using an enum field (per `kube-api-linter` `nobools` convention).
- Authentication of end-users via OIDC (`--oidc-*` flags / `AuthenticationConfiguration` file) — covered by the separate `authentication` feature (NCN-118972). The two features compose additively as long as their issuer URLs do not overlap.
- EKS support — EKS does not expose API server flags (`ManagedControlPlane`). Document as kubeadm-only.

## Other ServiceAccount API server flags — coverage check

Current implementation covers 4 of ~8 kube-apiserver ServiceAccount flags:

| Flag | Supported | Notes |
|------|-----------|-------|
| `--service-account-issuer` | ✅ (`issuer`) | Required `https://` URL (no userinfo, query, or fragment). |
| `--api-audiences` | ✅ (`audiences`) | Comma-joined alphabetically sorted list (no whitespace, commas, or quotes). |
| `--service-account-jwks-uri` | ✅ (`jwksURI`) | Optional `https://` override (no userinfo or fragment; query allowed). |
| `--service-account-max-token-expiration` | ✅ (`maxTokenExpiration`) | Go duration in `[1h, 1193046h28m16s]` (`[1h, 2^32s]`). |
| `--service-account-extend-token-expiration` | ❌ | Defaults to `true` in `kube-apiserver`, extending default admission-injected Pod projected tokens (`3607s`) up to 1y even when `maxTokenExpiration` is set; documented in user docs and additive as an enum field if requested. |
| `--service-account-lookup` | ❌ | Bool to enable ServiceAccount lookup for token validation; not requested; additive `lookup *bool` if needed. |
| `--service-account-key-file` / `--service-account-signing-key-file` | ❌ | Key management; kubeadm generates keys; out of scope per spec, additive `signingKeyRef` style if custom rotation needed. |
| `--service-account-legacy-*` | ❌ | Deprecated legacy flags; not supported. |

All uncovered flags are additive optional fields on the same `ServiceAccountIssuer` type — no migration needed to add later. Conflict handling would extend to them in `FR-006`.

## Open Questions

- None — scope now covers issuer + audiences + jwksURI + maxTokenExpiration + provider-agnostic e2e (no external deps). Remaining flags (key files, lookup, extend) stay out of scope pending demand.

## Design Decisions

| Decision | Choice | Reason |
|---|---|---|
| Config surface | `apiServer.extraArgs` flags (`service-account-issuer`, `api-audiences`, `service-account-jwks-uri`, `service-account-max-token-expiration`) | kubeadm's supported surface; no file needed; matches `extraAPIServerCertSANs` handler simplicity. |
| API location | `KubeadmClusterConfigSpec.serviceAccountIssuer` in `api/v1alpha1/` | Provider-agnostic, alongside other API server config; follows `podSecurityAdmission` / `encryptionAtRest` / `authentication` precedent. |
| Handler | New `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/` | Provider-agnostic; reuses `patches.MutateIfApplicable` + `selectors.ControlPlane()` + `variables.Get` pattern; no new shared helper needed. |
| Scope | Kubeadm providers only (AWS, Docker, Nutanix) | EKS has no API server flag access; documented explicitly. |
| Validation | CRD OpenAPI + CEL (kubebuilder markers) | Immediate admission feedback; same technique as `kubelet_types.go` / `pod_security_admission_types.go`. |
| Conflict handling | Error on duplicate extraArgs | Prevents silent fighting between direct extraArgs and typed field; clear UX. |
| Sorting | Delegate to existing `sortextraargs` handler | Already ensures stable `extraArgs` ordering; no custom sort needed. |
| Key files | Out of scope | kubeadm-managed keys are sufficient for MVP; keep API minimal and additive. |
