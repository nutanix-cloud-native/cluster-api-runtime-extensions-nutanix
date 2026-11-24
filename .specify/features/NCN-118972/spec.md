<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Configure Kubernetes API server authentication via AuthenticationConfiguration file (OIDC/JWT)

**Jira Ticket**: [NCN-118972](https://jira.nutanix.com/browse/NCN-118972)
**Feature Branch**: `NCN-118972-authentication-config`
**Created**: 2026-11-05
**Status**: Draft
**Input**: User description: "allows configuration of the OIDC issuer flags of the Kubernetes API server" — implemented via the modern **AuthenticationConfiguration file approach** (`--authentication-config`, KEP-3331) instead of the deprecated `--oidc-*` flags, per user direction.

## Background

### Why a file, not flags

Kubernetes introduced **Structured Authentication Configuration** (KEP-3331, `apiserver.config.k8s.io/v1` GA in Kubernetes 1.34+). The API server accepts a single file flag:

```
--authentication-config=/etc/kubernetes/authentication.yaml
```

whose content is an `AuthenticationConfiguration` (`apiserver.config.k8s.io/v1`) document containing an ordered list of `jwt` authenticators. This replaces the individual flags (`--oidc-issuer-url`, `--oidc-client-id`, `--oidc-ca-file`, `--oidc-username-claim`, `--oidc-groups-claim`, `--oidc-required-claim`, etc.). The two forms are **mutually exclusive**: when `--authentication-config` is set no `--oidc-*` flag may be present (and vice versa). The file is re-read every minute and hot-reloaded; the `--oidc-*` flags are deprecated (their KEP calls out removal intent).

CAREN currently has **no way** to configure API server OIDC/JWT authentication. Users must hand-edit the `KubeadmControlPlaneTemplate` or front the cluster with an external proxy. The only CAREN-managed API server configuration surfaces are the pod-security, encryption-at-rest, and admission-configuration helpers (which also use file-based config + `--admission-control-config-file`).

### Existing patterns in CAREN to reuse

* `api/v1alpha1/KubeadmClusterConfigSpec` — the provider-agnostic place for API server-related config (already holds `kubernetesImageRepository`, `etcd`, `encryptionAtRest`, `dns`, `podSecurityAdmission`).
* `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration` — a shared helper that (1) creates/updates a YAML file on the control plane node via `bootstrapv1.File`, (2) mounts it into the API server via `APIServer.ExtraVolumes`, and (3) sets an API server flag. This handler is opt-in and a no-op when its variable is absent.
* `podSecurityAdmission` / `encryptionAtRest` — existing handlers that follow the same opt-in file + flag pattern.

### Scope decision

Cover the full **JWT authenticator** surface defined by `apiserver.config.k8s.io/v1` rather than just the historic `--oidc-*` subset, because the file is the only supported configuration surface and the extra surface (claim validation rules, CEL expressions, etc.) is what makes it useful. The CAREN API types mirror the upstream spec so we do not invent a lossy abstraction.

---

## User Scenarios & Testing

### User Story 1 — Operator configures OIDC/JWT authentication for a cluster (Priority: P1)

An operator wants OIDC login for a workload cluster backed by an external identity provider (IdP). They add an `authentication` block to their `KubeadmClusterConfigSpec` with one or more JWT authenticators (issuer URL + audiences + claim mappings). On next reconciliation the control plane API server starts serving with the OIDC configuration, and OIDC tokens issued by the IdP are accepted.

**Why this priority**: This is the core capability requested — configuring the API server's OIDC issuer via CAREN.

**Independent Test**: Apply a Cluster whose `clusterConfig.authentication.jwt` lists a single authenticator (issuer `https://issuer.example.com`, audience `k8s`, username claim `email`). Verify the generated `KubeadmControlPlaneTemplate` contains (a) a file at `/etc/kubernetes/authentication.yaml` with that authenticator and (b) `APIServer.ExtraArgs` includes `--authentication-config=/etc/kubernetes/authentication.yaml` and no `--oidc-*` flags. (Unit + handler integration test.)

**Acceptance Scenarios**:

1. **Given** a Cluster with `clusterConfig.authentication.jwt` set to one authenticator (URL `https://issuer.example.com`, audiences `["k8s"]`, username `email`, groups `groups`), **When** the topology mutation handler runs, **Then** it adds a file `/etc/kubernetes/authentication.yaml` whose YAML parses as `AuthenticationConfiguration` with those values, adds an `ExtraVolumes` entry for that path, and sets `--authentication-config` to that path.
2. **Given** a Cluster with two authenticators (different issuer URLs), **When** the handler runs, **Then** the rendered file preserves their order and both authenticate correctly (the file ordering is the API server's evaluation order).
3. **Given** a Cluster that previously had two authenticators and is updated to one, **When** the handler runs, **Then** the file is overwritten to contain exactly that one authenticator (no stale entries).

---

### User Story 2 — Upgrading CAREN is a no-op for existing clusters (Priority: P1)

An operator upgrades CAREN on a management cluster with many workload clusters that do **not** set the new `authentication` field. No workload cluster rolls.

**Why this priority**: Constitution principle VII — handler version safety. The feature must be opt-in and invisible when unset.

**Independent Test**: For a representative Cluster that does not set `authentication`, compare `GeneratePatches` output of the new handler (and the control-plane pipeline overall) before and after the change. Output must be byte-identical. Confirm no handler version bump is required.

**Acceptance Scenarios**:

1. **Given** an existing Cluster without `authentication` set, **When** CAREN is upgraded to the version containing this feature, **Then** neither its `KubeadmControlPlaneTemplate` nor its `ClusterClass` patches change, and no Machine rollout is triggered.
2. **Given** the PR diff, **When** reviewing, **Then** there is no bump of any existing handler's version name/string introduced solely by this change.

---

### User Story 3 — Operator uses the full JWT authenticator surface (Priority: P2)

An operator needs audience matching policies, per-claim validation rules, CEL expressions for claim mapping, and user validation rules — capabilities only available via the structured file (not via flags).

**Why this priority**: This is the reason to prefer the file: it unlocks multi-audience, claimValidationRules, and userValidationRules that flags cannot express. P2 because basic OIDC (P1) is useful without them, but the API should not be artificially narrow.

**Independent Test**: Apply a Cluster with an authenticator that sets `issuer.audienceMatchPolicy`, two `claimValidationRules` (one claim+requiredValue, one CEL expression), a CEL-backed `claimMappings.username`, and a `userValidationRule`. Verify every field round-trips into the rendered YAML.

**Acceptance Scenarios**:

1. **Given** an authenticator whose `issuer` sets `audiences: ["a","b"]` and `audienceMatchPolicy: MatchAny`, **When** rendered, **Then** the YAML contains exactly those values.
2. **Given** an authenticator with `claimValidationRules: [{claim: "hd", requiredValue: "example.com"}, {expression: 'claims.exp - claims.nbf <= 86400'}]`, **When** rendered, **Then** both rules appear.
3. **Given** an authenticator whose `claimMappings.username` uses `expression: 'claims.email + ":ext"'` and whose `claimMappings.extra` lists a key/valueExpression pair, **When** rendered, **Then** the CEL and extra entries appear.

---

### User Story 4 — Invalid authentication configuration is rejected early (Priority: P2)

An operator typos an issuer URL or omits required fields. They get a clear error at Cluster admission/validation time (CRD CEL / webhook) rather than a broken API server after rollout.

**Why this priority**: Without validation a broken file would make the API server keep serving the last good config (per upstream semantics) but the operator would not know until they check control plane logs. Early rejection is the expected UX in CAREN (see `PodSecurityAdmission`, `ETCD`, `EncryptionAtRest` validation patterns).

**Independent Test**: Apply Cluster YAML that violates each required-field / format constraint and assert admission is denied with a message naming the field. Additionally, verify in the handler that a structurally-valid but unusual config still renders (upstream will do semantic validation).

**Acceptance Scenarios**:

1. **Given** a Cluster YAML whose `authentication.jwt[0].issuer.url` is empty or is not an `https://` URL, **When** applied, **Then** admission is rejected.
2. **Given** a Cluster YAML whose authenticator has no `audiences` entries or whose `claimMappings.username` has neither `claim` nor `expression`, **When** applied, **Then** admission is rejected.
3. **Given** a valid Cluster that sets the field, **When** it is later patched to remove one required field, **Then** the update is rejected.

---

### User Story 5 — Operator end-to-end validates JWTs via in-cluster mock OIDC (Priority: P1)

An operator configures `clusterConfig.authentication` to trust a mock OIDC issuer deployed **inside the workload cluster** (no external service). Tokens minted by that issuer are accepted by the API server with the configured claim mappings; invalid tokens are rejected. The same e2e proves the feature does not break admin cert auth.

**Why this priority**: Proves the full loop — CAREN renders the file, mounts it, sets `--authentication-config`, API server hot-reloads and validates JWTs — without depending on an external IdP. Extra e2e time is acceptable.

**Independent Test** (provider-agnostic, runs as part of normal e2e on any provider, not skipped): `test/e2e/quick_start_test.go` (`topology-cilium-helm-addon` flavor) + untagged helpers in `test/framework/oidc/`. Steps:
1. Resolve the workload Cluster's Service CIDR from its template (`spec.clusterNetwork.services.cidrBlocks`), pick a deterministic static `ClusterIP` in Kubernetes's static Service IP band, pre-generate an ephemeral RSA signing key, self-signed CA, and TLS server certificate with an IP SAN for that `ClusterIP`, and inject `clusterConfig.authentication` (`issuer.url=https://<ClusterIP>`, `discoveryURL=https://<ClusterIP>/.well-known/openid-configuration`, `audiences:["e2e-aud"]`, `audienceMatchPolicy:MatchAny`, `certificateAuthority:<CA PEM>`, `claimMappings.username:{claim:email,prefix:""}`, `groups:{claim:groups,prefix:""}`, `extra:[{key:example.org/foo,valueExpression:claims.foo}]`, `claimValidationRules:[{claim:hd,requiredValue:example.com}]`, `userValidationRules:[{expression:!user.username.startsWith('system:')}]`) into the Cluster template before provisioning so control-plane nodes boot once with no day-2 KCP replacement.
2. Provision the workload Cluster → `Ready`; verify the live Cluster's Service CIDR contains the static `ClusterIP` and `KubeadmControlPlane` carries `/etc/kubernetes/authentication.yaml`.
3. Deploy the mock OIDC `Deployment/Service` (`spec.clusterIP: <ClusterIP>`, port `443`, `https`) into the workload cluster; `kube-apiserver`'s asynchronous OIDC discovery loop initializes the verifier once `mock-oidc` becomes `Available`.
4. Mint valid JWT locally with the pre-generated RSA private key (`iss` = issuer URL, `aud` = `e2e-aud`, `email` = `test@example.com`, `groups` = `["e2e-group"]`, `hd` = `example.com`, `foo` = `bar`) → `SelfSubjectReview` with Bearer token confirms `username=test@example.com`, `groups` includes `e2e-group`, and `extra["example.org/foo"]=["bar"]`; invalid JWTs (wrong `aud`, `hd`, expired, `system:` username) → `401`; admin kubeconfig still works.

**Acceptance Scenarios**:

1. **Given** a workload cluster with mock OIDC and valid `authentication` config, **When** a JWT with `email=test@example.com` and `aud=e2e-aud` is presented, **Then** the API server authenticates it as `test@example.com`.
2. **Given** the same cluster, **When** a JWT missing `hd=example.com` or with `aud` mismatch or expired `exp` is presented, **Then** the API server rejects it.
3. **Given** the same cluster, **When** admin cert auth is used, **Then** `Cluster` remains `Ready` and `kubectl` via admin kubeconfig still succeeds (no breakage).

---

### Edge Cases

- What happens when the user also sets legacy `--oidc-*` flags via `KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` directly? The handler **MUST fail the reconciliation** with a clear error when both are present; the API server would refuse to start if both were set. The error tells the user to remove the legacy flags and use `authentication` instead. (If the user later removes the file config, the legacy flags can be used again.)
- How does file rotation/hot-reload work? kube-apiserver polls the file every minute. CAREN delivers the file and `--authentication-config` by patching KubeadmControlPlane, which rolls control-plane machines. It does not edit the on-disk file in place.
- What if the CAREN-managed `AuthenticationConfiguration` coexists with a CAREN-managed `AdmissionConfiguration`? They use **different paths** (`/etc/kubernetes/authentication.yaml` vs `/etc/kubernetes/admission.yaml`), different `ExtraVolumes` mount names, and different flags (`--authentication-config` vs `--admission-control-config-file`). They compose additively (mirrors `podSecurityAdmission` + `encryptionAtRest` composition). When both set `authenticationConfig` and `admissionConfig` CAREN must not overwrite the other's file or flag.
- What happens when the CA bundle PEM is set? It is inlined as `certificateAuthority` string in the YAML, not mounted as a file. If a future need for a file-backed CA arises, it can be added without breaking this design.
- How does the feature handle clusters pinned to Kubernetes < 1.34 (where `apiserver.config.k8s.io/v1` is not served)? When `clusterConfig.authentication` is set, the control-plane mutation handler checks `builtin.controlPlane.version` (`>= 1.34.0-0`) and returns an error on older versions rather than rendering a `v1` file that `kube-apiserver` would refuse to start with.
- Removing the last authenticator (setting `authentication` back to absent/empty) — the handler emits no patches against the base `ClusterClass` `KubeadmControlPlaneTemplate`, consistent with all other handlers (e.g. `podSecurityAdmission`, `encryptionAtRest`). On a cluster that previously enabled `authentication`, Cluster API topology reconciliation recomputes `KubeadmControlPlane` from the unpatched template (removing the file, volume mount, and `--authentication-config` and rolling control-plane machines).

---

## Requirements

### Functional Requirements

- **FR-001**: The system MUST add an **opt-in** field `authentication` to `KubeadmClusterConfigSpec` (in `api/v1alpha1/`) that carries a list of JWT authenticators. When the field is unset or empty the feature is a no-op; `GeneratePatches` output for such inputs MUST be byte-identical to the pre-change output.
- **FR-002**: Each JWT authenticator MUST expose an `issuer` with at least `url` (https URL, required, unique within the list) and `audiences` (non-empty list of strings, required). It MAY expose `discoveryURL`, `certificateAuthority` (PEM bundle string), and `audienceMatchPolicy` (enum: empty or `MatchAny`) mirroring `apiserver.config.k8s.io/v1`'s `Issuer`.
- **FR-003**: Each authenticator MAY expose `claimValidationRules` (list of `{claim+requiredValue}` **or** `{expression+message}` entries, mutually exclusive per entry), `claimMappings` (with `username` required — `{claim}+prefix` **or** `{expression}` — plus optional `groups`, `uid`, `extra[]`), and `userValidationRules` (list of `{expression+message}`) matching the upstream JWTAuthenticator shape.
- **FR-004**: A **new provider-agnostic topology mutation handler** (under `pkg/handlers/generic/mutation/kubeadm/authentication/` or equivalent) MUST, when the variable is present and non-empty, (a) render a single file `AuthenticationConfiguration` at `/etc/kubernetes/authentication.yaml` (content is the YAML encoding of `apiVersion: apiserver.config.k8s.io/v1, kind: AuthenticationConfiguration, jwt: [...]`), (b) ensure `KubeadmControlPlaneTemplate.spec.template.spec.files` contains that file, (c) add an `ExtraVolumes` entry for its path, and (d) set `APIServer.ExtraArgs` `--authentication-config` to that path. It MUST NOT add any `--oidc-*` flag.
- **FR-005**: When the rendered file already exists (e.g. from a prior reconciliation), the handler MUST overwrite it idempotently with the current desired `jwt` list. When the variable is absent/empty the handler is a **no-op** (no patches); consistent with `podSecurityAdmission`/`encryptionAtRest` semantics, stale files are not actively deleted — they are simply no longer rendered.
- **FR-006**: If `APIServer.ExtraArgs` already contains `--authentication-config` (e.g. user set it directly) the handler SHOULD adopt it only if its value equals the well-known path; otherwise it MUST return an error rather than silently fight a user-managed conflicting path. If `ExtraArgs` already contains any `--oidc-*` flag, the handler MUST return an error explaining that `--oidc-*` flags and `--authentication-config` are mutually exclusive and that the user should remove the `--oidc-*` entries and use `authentication` instead. If any `authentication.jwt[i].issuer.url` overlaps `--service-account-issuer` in `APIServer.ExtraArgs` (or kubeadm's default `https://kubernetes.default.svc.<dnsDomain>` when `--service-account-issuer` is unset), validation MUST reject the configuration.
- **FR-007**: API types MUST carry kubebuilder/CEL validation: `jwt` required non-empty when `authentication` is present; `issuer.url` required and `https://` URL pattern, unique within the list; `audiences` required non-empty, unique; at least one of `claim`/`expression` set where the upstream API says "either/or"; list size caps to bound input (align with `PodSecurityAdmission.MaxItems` precedent). In addition, `/validate-cluster` (`pkg/webhook/cluster/`) MUST run semantic validation (`>= 1.34.0` Kubernetes version check, upstream `ValidateAuthenticationConfiguration` including `--service-account-issuer` overlap, and `KubeadmControlPlaneTemplate` conflict checks when the referenced `ClusterClass` and template exist) so invalid configurations fail immediately at `Cluster` admission.
- **FR-008**: The rendered file MUST be deterministic and stable (sorted only where order is semantically irrelevant; `jwt` order is preserved as the user's declared evaluation order; within a single authenticator, `audiences` order is preserved).
- **FR-009**: The feature MUST be documented under `docs/content/customization/kubeadm/authentication.md` with what it does, how to enable it, examples for single-issuer OIDC, multi-issuer, and CEL rules, a mutual-exclusivity note vs `--oidc-*`, the minimum Kubernetes version (`>= 1.34`), and example YAML.
- **FR-010**: An **integration/e2e test** MUST exist that validates the full loop — CAREN renders/mounts the file and sets `--authentication-config`, workload Cluster reaches `Ready`, and a JWT minted by an **in-cluster mock OIDC** (no external service dependency) is accepted/rejected by the API server per the configured claim mappings/validation. The test is provider-agnostic and runs as part of normal e2e on any provider (no `provider:` filter, no opt-in skip).

### Key Entities

- **KubeadmClusterConfigSpec.authentication**: Opt-in block on the provider-agnostic KubeadmClusterConfig that carries the desired JWT authentication configuration. Lives in `api/v1alpha1/` alongside `podSecurityAdmission`, `encryptionAtRest`, `etcd`, etc.
- **JWTAuthenticator**: One OIDC/JWT authenticator (issuer + audiences + optional claim/user validation and claim mappings), mirroring the upstream `apiserver.config.k8s.io/v1` type. Multiple authenticators form an ordered list; order is the API server's evaluation order.
- **AuthenticationConfiguration file**: The `apiserver.config.k8s.io/v1` YAML document written to `/etc/kubernetes/authentication.yaml` on the control plane hosts, mounted into the API server, and referenced by `--authentication-config`.
- **Authentication mutation handler**: The provider-agnostic handler that turns the variable into file + mount + flag patches on `KubeadmControlPlaneTemplate`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: An operator can add `clusterConfig.authentication.jwt` with a single issuer (URL + audience + username claim) and the resulting `KubeadmControlPlaneTemplate` contains a file at `/etc/kubernetes/authentication.yaml` whose YAML decodes to `AuthenticationConfiguration` with that issuer, a matching `ExtraVolumes` entry, and `--authentication-config` set to that path — with no `--oidc-*` flag present.
- **SC-002**: 100% of inputs that did **not** set `authentication` produce byte-identical `GeneratePatches` output to the pre-change behavior.
- **SC-003**: Any Cluster YAML that sets `authentication` but omits or mis-formats a required field (issuer URL, audiences, username mapping) is rejected at admission with a message naming the offending field.
- **SC-004**: Documentation page is published at `docs/content/customization/kubeadm/authentication.md` and linked from the customization index, with a working single-issuer example that can be copied directly.
- **SC-005**: A Cluster that sets legacy `--oidc-*` `ExtraArgs` and also sets `clusterConfig.authentication` receives a clear handler error rather than a broken API server rollout.
- **SC-006**: A workload Cluster (any provider) configured with an in-cluster mock OIDC reaches `Ready`, and a valid JWT minted by that mock issuer is authenticated by the API server (username/groups/extra per `claimMappings`) while invalid JWTs are rejected — with no external service dependency, as part of normal `make e2e-test` (no `provider:` filter).

## Out of Scope

- Managing a production IdP, Dex, `kube-oidc-proxy`, `ServiceAccount.issuer`, or `--service-account-*` flags — this feature concerns only the API server's **authentication** of OIDC/JWT bearer tokens via `AuthenticationConfiguration`. An **in-cluster mock OIDC** is in-scope for e2e.
- `anonymous` and `webhook` authenticator subsections of `AuthenticationConfiguration` — the initial implementation covers `jwt`. Those subsections can be added later in the same file without changing the API surface shape (additive, opt-in list entries).
- Automated discovery/JWKS pre-validation at admission time (network reachability checks to the IdP) — validation covers structural correctness only.

## Open Questions

*None — resolved via background research. Any remaining uncertainty (exact CEL markers needed for "exactly one of claim/expression") is deferred to plan phase after inspecting `kubelet_types.go` / `pod_security_admission_types.go` patterns.*

## Design Decisions

| Decision | Choice | Reason |
|---|---|---|
| Config surface | `AuthenticationConfiguration` file (`--authentication-config`) | Only supported surface going forward; `--oidc-*` deprecated and mutually exclusive; file unlocks multi-issuer + CEL rules. |
| API location | `KubeadmClusterConfigSpec.authentication` in `api/v1alpha1/` | Provider-agnostic, alongside other API server file-driven configs; follows `encryptionAtRest`/`podSecurityAdmission` precedent. |
| Handler | New `pkg/handlers/generic/mutation/kubeadm/authentication/` | Reuses the admission-configuration helper pattern (file + mount + flag) without cross-provider leakage. |
| Username/groups mapping | `claim`+`prefix` vs `expression` per field | Mirrors upstream `PrefixedClaimOrExpression`/`ClaimOrExpression` so no semantic loss vs raw file. |
| Validation | CRD CEL markers + OpenAPI schema | Immediate feedback; same technique as `kubelet_types.go` in this repo. |
| E2E validation | In-cluster mock OIDC (`https` + CA via `issuer.certificateAuthority`) | Proves file+flag+hot-reload+JWT validation without external service; provider-agnostic, runs as part of normal e2e on any provider. |
