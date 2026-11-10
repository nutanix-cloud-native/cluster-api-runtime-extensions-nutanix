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
2. **Given** a Cluster with `issuer: https://oidc.example.com` and `audiences: ["sts.amazonaws.com", "k8s.example.com"]`, **When** handled, **Then** it adds both `service-account-issuer=https://oidc.example.com` and `api-audiences=sts.amazonaws.com,k8s.example.com`.
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
2. **Given** an issuer with `audiences: ["a", "b", "c"]`, **When** rendered, **Then** `api-audiences` equals `a,b,c` in declared order (preserved).
3. **Given** an issuer without `audiences`, **When** rendered, **Then** no `api-audiences` flag is set (API server defaults to issuer URL).

### Edge Cases

- What happens when the user also sets `service-account-issuer` or `api-audiences` directly via `KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` (out-of-band)? The handler MUST detect duplicate/conflicting extraArgs and return a clear error rather than silently duplicate or fight the user's direct setting. The error tells the user to remove the direct extraArgs and use `serviceAccountIssuer` instead. This mirrors the `authentication` handler's handling of `--oidc-*` / `--authentication-config` conflicts.
- What happens when an existing cluster already has `service-account-issuer` set to the default `https://kubernetes.default.svc.cluster.local` (kubeadm default) and the operator sets a new issuer? The handler overwrites with the new value; the change **will** trigger a control-plane rollout, which is expected and desired (issuer is immutable for issued tokens).
- What happens when the issuer URL points to an external OIDC provider that is not yet reachable at API server start? The API server still starts; token issuance uses that issuer string. OIDC discovery (`/.well-known/openid-configuration`) is served by the API server itself based on the issuer URL, not by dialing the external provider. No handler-level reachability check is needed.
- How does this compose with `AuthenticationConfiguration` (NCN-118972) or `PodSecurityAdmission` or `EncryptionAtRest`? They use different flags/files/paths (`--authentication-config`, `--admission-control-config-file`, `encryption-provider-config`, etc.) and compose additively. The `serviceAccountIssuer` handler only touches `service-account-issuer`, `api-audiences`, and optionally `service-account-jwks-uri`.
- Removing the field (setting `serviceAccountIssuer` back to nil/empty) makes the handler a **no-op** (consistent with `podSecurityAdmission`, `encryptionAtRest`, `authentication`). Stale extraArgs are not actively deleted by the handler; absence of the variable produces no patches. Operators needing to revert to kubeadm defaults should remove the field and, if needed, manually reconcile.
- Key file management (`--service-account-key-file`, `--service-account-signing-key-file`) is out of scope for this feature. kubeadm already manages a signing key pair; custom key rotation is not covered initially.

## Requirements

### Functional Requirements

- **FR-001**: The system MUST add an **opt-in** field `serviceAccountIssuer` to `KubeadmClusterConfigSpec` in `api/v1alpha1/` (alongside `podSecurityAdmission`, `encryptionAtRest`, `etcd`, `dns`, `authentication`). When the field is unset/nil the feature is a no-op; `GeneratePatches` output for such inputs MUST be byte-identical to pre-change output.
- **FR-002**: The `serviceAccountIssuer` type MUST expose `issuer` (required, non-empty, must be an `https://` URL compliant with RFC 3986, max length 2048, matching OpenID Discovery issuer requirements). It MUST carry kubebuilder validation (`Required`, `Format=uri`, `Pattern=^https://`).
- **FR-003**: The type MAY expose `audiences` (optional, list of non-empty strings, each 1-253 chars, max 50 entries, unique items). When set, the handler MUST set `--api-audiences` to the comma-joined list in declared order. When absent/empty, the handler MUST NOT set `--api-audiences` (API server defaults to issuer).
- **FR-004**: The type MAY expose `jwksURI` (optional, when set must be an `https://` URL, max length 2048). When set, the handler MUST set `--service-account-jwks-uri` to that value. This is useful when discovery doc and JWKS are served from a different external URL.
- **FR-005**: A **new provider-agnostic topology mutation handler** under `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/` (or equivalent) MUST, when the variable is present and non-nil, add/update `KubeadmControlPlaneTemplate.spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs` entries for `service-account-issuer` (always) and optionally `api-audiences` and `service-account-jwks-uri`. It MUST be idempotent and MUST NOT add any other flags.
- **FR-006**: If `apiServer.extraArgs` already contains `service-account-issuer`, `api-audiences`, or `service-account-jwks-uri` (user set directly), the handler MUST return an error explaining the conflict and directing the user to remove the direct extraArgs and use `serviceAccountIssuer` instead. It MUST NOT silently overwrite.
- **FR-007**: API types MUST carry kubebuilder / CEL validation: `issuer` required when `serviceAccountIssuer` is present; `issuer` and `jwksURI` must be `https://` URLs; `audiences` maxItems 50, unique, each item minLength 1, maxLength 253; the whole block is optional (pointer, nil = absent).
- **FR-008**: The rendered `extraArgs` MUST be deterministic and stable. Ordering is delegated to the existing `sortextraargs` handler (which sorts `apiServer.extraArgs` by name) — the new handler does not need to sort, but MUST be registered **before** `sortextraargs` in `MetaMutators()` so final output is stable.
- **FR-009**: The feature MUST be available to **kubeadm-based providers only** (AWS, Docker, Nutanix). EKS is excluded (managed control plane — no access to API server flags). This matches `podSecurityAdmission` and `encryptionAtRest` scoping.
- **FR-010**: The feature MUST be documented under `docs/content/customization/kubeadm/` (new page `service-account-issuer.md` plus `kubeadm/_index.md` link) with configuration tables, what it does, how to enable it, examples for single issuer and with audiences/JWKS URI, provider scoping note, and minimum Kubernetes version note. Documentation MUST note that changing the issuer triggers a control-plane rollout and that tokens issued before the change remain valid until expiry.

### Key Entities

- **KubeadmClusterConfigSpec.serviceAccountIssuer**: Opt-in block on the provider-agnostic KubeadmClusterConfig that carries the desired ServiceAccount token issuer configuration. Lives in `api/v1alpha1/` alongside other API server-related config. Affects `KubeadmControlPlaneTemplate` only (control plane).
- **ServiceAccountIssuer**: One issuer configuration: required `issuer` URL + optional `audiences` list + optional `jwksURI` override. Maps 1:1 to API server flags `service-account-issuer`, `api-audiences`, `service-account-jwks-uri`.
- **ServiceAccount mutation handler**: Provider-agnostic handler that turns the variable into `apiServer.extraArgs` patches on `KubeadmControlPlaneTemplate`. Registered in `pkg/handlers/generic/mutation/handlers.go` `MetaMutators()`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: An operator can add `clusterConfig.serviceAccountIssuer.issuer: https://oidc.example.com` and the resulting `KubeadmControlPlaneTemplate` contains `extraArgs` with `service-account-issuer=https://oidc.example.com` and no other new flags; the patch is applied on next reconciliation and the API server serves discovery at that issuer.
- **SC-002**: With `audiences: ["sts.amazonaws.com", "k8s.example.com"]` the resulting `api-audiences` equals `sts.amazonaws.com,k8s.example.com` (order preserved).
- **SC-003**: 100% of inputs that did **not** set `serviceAccountIssuer` produce byte-identical `GeneratePatches` output to pre-change behavior (no rollout on upgrade).
- **SC-004**: Any Cluster YAML that sets `serviceAccountIssuer` but omits or mis-formats `issuer` (empty, non-https, too long) is rejected at admission with a message naming the offending field.
- **SC-005**: Documentation page is published at `docs/content/customization/kubeadm/service-account-issuer.md` and linked from the customization index, with a working example that can be copied directly.
- **SC-006**: A Cluster that sets `service-account-issuer` or `api-audiences` directly via `extraArgs` and also sets `serviceAccountIssuer` receives a clear handler error rather than a silent conflict or broken API server rollout.

## Out of Scope

- Managing the IdP / OIDC provider itself, Dex, `kube-oidc-proxy`, or cloud IAM role bindings — this feature only configures the API server's issuer identity, not the consumer side.
- `--service-account-key-file` / `--service-account-signing-key-file` custom key management and rotation — kubeadm already generates and rotates a key pair; custom key injection can be added later if needed without breaking this API (additive optional fields).
- `--service-account-extend-token-expiration` / `--service-account-max-token-expiration` lifetime tuning — can be added later as optional fields on the same type (additive, no migration).
- Authentication of end-users via OIDC (`--oidc-*` flags / `AuthenticationConfiguration` file) — covered by the separate `authentication` feature (NCN-118972). The two features compose but are independent.
- EKS support — EKS does not expose API server flags (`ManagedControlPlane`). Document as kubeadm-only.

## Open Questions

- None — scope deliberately minimal (issuer + audiences + jwksURI) to match immediate workload-identity need. Additional flags (key files, token lifetimes) can be added additively if requested.

## Design Decisions

| Decision | Choice | Reason |
|---|---|---|
| Config surface | `apiServer.extraArgs` flags (`service-account-issuer`, `api-audiences`, `service-account-jwks-uri`) | kubeadm's supported surface; no file needed; matches `extraAPIServerCertSANs` handler simplicity. |
| API location | `KubeadmClusterConfigSpec.serviceAccountIssuer` in `api/v1alpha1/` | Provider-agnostic, alongside other API server config; follows `podSecurityAdmission` / `encryptionAtRest` / `authentication` precedent. |
| Handler | New `pkg/handlers/generic/mutation/kubeadm/serviceaccountissuer/` | Provider-agnostic; reuses `patches.MutateIfApplicable` + `selectors.ControlPlane()` + `variables.Get` pattern; no new shared helper needed. |
| Scope | Kubeadm providers only (AWS, Docker, Nutanix) | EKS has no API server flag access; documented explicitly. |
| Validation | CRD OpenAPI + CEL (kubebuilder markers) | Immediate admission feedback; same technique as `kubelet_types.go` / `pod_security_admission_types.go`. |
| Conflict handling | Error on duplicate extraArgs | Prevents silent fighting between direct extraArgs and typed field; clear UX. |
| Sorting | Delegate to existing `sortextraargs` handler | Already ensures stable `extraArgs` ordering; no custom sort needed. |
| Key files | Out of scope | kubeadm-managed keys are sufficient for MVP; keep API minimal and additive. |
