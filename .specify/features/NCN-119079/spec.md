<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Support http schemes for container registries by properly configuring containerd

**Jira Ticket**: [NCN-119079](https://jira.nutanix.com/browse/NCN-119079)
**Feature Branch**: `NCN-119079-support-http-registry-containerd`
**Created**: 2026-11-11
**Status**: Draft
**Input**: User description: "support http schemes for container registries by properly configuring containerd"

## User Scenarios & Testing

### User Story 1 - Global mirror with http scheme is properly configured (Priority: P1)

An operator runs an air-gapped or internal registry that is only accessible over plain HTTP (no TLS). They configure `clusterConfig.globalImageRegistryMirror.url` with an `http://` URL (e.g. `http://10.99.0.10:5000`). The system must configure containerd on all nodes to pull images through that mirror over HTTP.

**Why this priority**: Global mirror is the primary mechanism for air-gapped installations and large-scale image caching. Without correct containerd configuration, image pulls fail even though the API accepts `http://` URLs. This is the most common http use-case.

**Independent Test**: Create a Cluster with `globalImageRegistryMirror.url: http://10.99.0.10:5000` (with or without authentication credentials). After mutation, inspect the generated `KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate` patches. Verify that `/etc/containerd/certs.d/_default/hosts.toml` contains a `[host."http://10.99.0.10:5000/v2"]` entry with `capabilities = ["pull", "resolve"]` and `override_path = true`, and that `/etc/caren/containerd/patches/registry-config.toml` sets `config_path = "/etc/containerd/certs.d"`. On a live node, `crictl pull` through the mirror succeeds over HTTP.

**Acceptance Scenarios**:

1. **Given** a Cluster with `globalImageRegistryMirror.url: http://my-mirror.example.com:5000`, **When** the generic mirror handler mutates the ClusterClass templates, **Then** the resulting files include `/etc/containerd/certs.d/_default/hosts.toml` with `host."http://my-mirror.example.com:5000/v2"` and no CA line.
2. **Given** the same with `http://my-mirror.example.com:5000/myproject`, **When** mutated, **Then** hosts.toml contains `host."http://my-mirror.example.com:5000/v2/myproject"` with correct path handling.
3. **Given** the same with a credentials secret containing `ca.crt`, **When** validated or mutated, **Then** the configuration is rejected because TLS CA certificates cannot be used with plain HTTP registries.

---

### User Story 2 - Per-registry http configuration for ImageRegistries (Priority: P1)

An operator configures `clusterConfig.imageRegistries` with one or more registries that use `http://` (e.g. insecure internal registries `http://registry.internal:5000`). The system must configure containerd so that pulls from those specific hosts use plain HTTP instead of HTTPS.

**Why this priority**: `imageRegistries` is used for authenticated pulls and CA injection. Insecure registries require per-host containerd configuration, not just global mirror. Without this, kubelet credential provider may be set up but containerd still attempts HTTPS and fails.

**Independent Test**: Create a Cluster with `imageRegistries: [{url: http://registry.internal:5000, credentials: {secretRef: {name: my-creds}}}]` where the secret contains `username`/`password` (without `ca.crt`). Verify mutated templates contain per-host containerd configuration under `/etc/containerd/certs.d/registry.internal:5000/hosts.toml` with a `[host."http://registry.internal:5000"]` entry and `capabilities = ["pull", "resolve"]`. Verify that if the secret contains `ca.crt`, admission and mutation reject the configuration. On a node, pulling `http://registry.internal:5000/myimage:tag` succeeds.

**Acceptance Scenarios**:

1. **Given** `imageRegistries` with `url: http://registry.internal:5000`, **When** mutated, **Then** the output includes a file at `/etc/containerd/certs.d/registry.internal:5000/hosts.toml` (and registry-config drop-in) and does NOT include `_default/hosts.toml` unless a global mirror is also set.
2. **Given** `imageRegistries` with mixed `http://` and `https://` hosts (distinct hosts), **When** mutated, **Then** only `http://` hosts get per-host hosts.toml, and `https://` hosts get no hosts.toml but still get `ca.crt` files if CA is present.
3. **Given** `imageRegistries` with `http://registry.internal:5000/myproject` (path component), **When** mutated, **Then** the per-host hosts.toml targets the host `registry.internal:5000` (`[host."http://registry.internal:5000"]`).
4. **Given** `imageRegistries` with `url: http://registry.internal:5000` and a credentials secret containing `ca.crt`, **When** validated or mutated, **Then** the configuration is rejected.

---

### User Story 3 - HTTPS registries continue to work unchanged (Priority: P1)

An operator with existing `https://` registry configuration upgrades CAREN. Their clusters must not experience unexpected rollouts or behavioral changes.

**Why this priority**: Constitution principle VII (no silent rollouts). The fix must be invisible to the dominant `https://` use-case.

**Independent Test**: Compare `GeneratePatches` output for a representative Cluster with `globalImageRegistryMirror.url: https://my-mirror.example.com` and `imageRegistries: [{url: https://my-registry.io}]` before and after the change. Output must be byte-identical (or limited to additive http-only paths that do not affect https inputs). Verify via unit test golden files.

**Acceptance Scenarios**:

1. **Given** a Cluster with only `https://` registry URLs, **When** CAREN is upgraded, **Then** the generated files (`_default/hosts.toml`, `ca.crt` files, `registry-config.toml`) are identical to the previous version and no Machine rollout is triggered.
2. **Given** a Cluster with no registry variables, **When** mutated, **Then** no containerd files are added (nil output), same as before.

---

### User Story 4 - Upgrade is a no-op for managed Clusters without http usage (Priority: P1)

An operator upgrades CAREN on a management cluster with many running workload clusters that do not use `http://` registries. They expect zero churn.

**Why this priority**: Constitution principle VII. This fix should not bump handler version if https behavior is unchanged; it is a bug-fix for a previously non-functional code path.

**Independent Test**: Run the existing mirror and credentials unit tests plus a new `GeneratePatches` comparison for https-only inputs. Confirm byte-identical output.

**Acceptance Scenarios**:

1. **Given** an existing Cluster that does not set any `http://` URL, **When** CAREN is upgraded, **Then** no `KubeadmControlPlaneTemplate` or `KubeadmConfigTemplate` is rolled.

### Edge Cases

- What happens when `http://` and `https://` URLs share the same host in `imageRegistries` (e.g. `http://registry.example.com` and `https://registry.example.com`)? Containerd's per-host namespace is keyed by `host:port`, so mixed schemes for the same host would be ambiguous. Both the Cluster admission webhook and the mutation handler reject conflicting schemes on the same host (`ErrConflictingRegistrySchemes`).
- What happens when an `http://` URL (`globalImageRegistryMirror` or `imageRegistries`) references a credentials secret containing `ca.crt`? Plain HTTP does not use TLS, and configuring a CA certificate on an `http://` host in containerd causes containerd to treat TLS as configured and attempt HTTPS first on non-80 ports. The Cluster admission webhook, preflight check, and mutation handlers MUST reject any `http://` registry whose credentials secret contains `ca.crt`.
- Does containerd need `skip_verify = true` (`insecureSkipVerify`) for `http://` registries? No. `skip_verify` only applies to TLS certificate verification on `https://` endpoints; in containerd `hosts.go`, setting `skip_verify` on an `http://` host marks TLS as explicitly configured and causes containerd to try HTTPS first on non-80 ports instead of plain HTTP.
- What happens when `globalImageRegistryMirror` is `http://` and `imageRegistries` also contains `http://` entries for different hosts? Both `_default/hosts.toml` and per-host hosts.toml files must be emitted together without overwriting each other.
- How does the containerd drop-in `registry-config.toml` interact? It must be emitted whenever any containerd configuration is needed (mirror or http host), same as today.
- What about registries without credentials secret? `http://` imageRegistries without credentials must still get per-host hosts.toml to enable plain HTTP and must not fail kubelet credential provider mutation with `ErrCredentialsNotFound`.

## Requirements

### Functional Requirements

- **FR-001**: System MUST accept `http://` and `https://` URLs for `GlobalImageRegistryMirror.url` and `ImageRegistry.url` (already validated via `^https?://` pattern) and correctly distinguish scheme.
- **FR-002**: When `GlobalImageRegistryMirror.url` has scheme `http`, system MUST generate `/etc/containerd/certs.d/_default/hosts.toml` with a `[host."http://<host>[:port]/v2[/path]"]` entry (preserving host, port, and path) with `capabilities = ["pull", "resolve"]` and `override_path = true`, without `ca` or `skip_verify`.
- **FR-003**: When `ImageRegistry.url` has scheme `http`, system MUST generate a per-registry hosts.toml at `/etc/containerd/certs.d/<host>/hosts.toml` (where `<host>` is `URL.Host`, including port if present) containing `[host."http://<host>"]` and `capabilities = ["pull", "resolve"]`, enabling plain HTTP pulls for that registry host without `skip_verify`.
- **FR-004**: System MUST reject any `http://` registry (`GlobalImageRegistryMirror` or `ImageRegistry`) whose credentials secret contains `ca.crt` in the Cluster admission webhook (when the secret exists), preflight check, and mutation handlers, while continuing to generate `/etc/containerd/certs.d/<host>/ca.crt` and detect conflicting CAs for `https://` registries.
- **FR-005**: System MUST emit the containerd registry config drop-in at `/etc/caren/containerd/patches/registry-config.toml` (`config_path = "/etc/containerd/certs.d"` for both CRI plugin paths) whenever any containerd configuration (mirror or http registry or CA) is required.
- **FR-006**: System MUST treat `https://` registries exactly as today: `GlobalImageRegistryMirror` via `_default/hosts.toml` with `https` scheme, `ImageRegistry` with CA via `ca.crt` file only (no per-host hosts.toml unless http), and not generate extra files for https imageRegistries without CA.
- **FR-007**: System MUST handle `ImageRegistry` with `http` URL that has no credentials secret (or secret without static credentials) by generating the per-host hosts.toml and skipping static kubelet credential configuration without returning `ErrCredentialsNotFound`.
- **FR-008**: System MUST not create duplicate, non-deterministic, or conflicting hosts.toml for the same host: multiple `ImageRegistry` entries sharing `URL.Host` with `http` map deterministically to a single per-host file; conflicting schemes on the same host fail with `ErrConflictingRegistrySchemes`; conflicting CA certs on the same `https` host fail with `ErrConflictingRegistryCACertificates`.
- **FR-009**: System MUST preserve existing helpers: `formatURLForContainerd` must correctly format both `http` and `https` URLs to `scheme://host/v2[/path]`; `filePathFromURL` must derive CA path from `URL.Host` regardless of scheme.
- **FR-010**: This change MUST NOT alter `GeneratePatches` output for inputs containing only `https://` URLs or no registry variables, ensuring upgrades are no-ops for existing clusters and per constitution VII no handler version bump is required (justified as bug-fix for previously non-functional http path).

### Key Entities

- **GlobalImageRegistryMirror**: Top-level variable specifying a default mirror for all registries; has URL (http/https) and optional credentials secret with CA.
- **ImageRegistry**: List variable specifying per-registry credentials/CA; each has URL (http/https) and optional secret.
- **ContainerdConfig**: Internal representation (`containerdConfig` struct) capturing URL, CASecretName, CACert, Mirror flag; extended to detect scheme.
- **Containerd hosts.toml**: File(s) under `/etc/containerd/certs.d/` that configure containerd registry access: `_default/hosts.toml` for global mirror, `<host>/hosts.toml` for per-registry http.
- **Containerd registry-config drop-in**: TOML patch at `/etc/caren/containerd/patches/registry-config.toml` setting `config_path`.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Clusters with `http://` global mirror generate correct `_default/hosts.toml` (verified via unit test golden files) and can pull images via HTTP in e2e/dev environment.
- **SC-002**: Clusters with `http://` imageRegistries generate per-host hosts.toml files at `/etc/containerd/certs.d/<host>/hosts.toml` (verified via unit tests) and pulls succeed; mixed http/https clusters generate correct combination of files without conflicts.
- **SC-003**: Existing unit tests for `https://` registries (containerd_files_test, inject_test) pass without modification to expected outputs, proving no regression for https.
- **SC-004**: Representative `GeneratePatches` outputs for https-only and empty inputs are byte-identical before and after, confirming upgrade is no-op.
- **SC-005**: Documentation under `docs/content/customization/generic/` is updated to mention `http://` support and example, and passes `make lint` and `pre-commit`.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Per-registry hosts file location | `/etc/containerd/certs.d/<host>/hosts.toml` with `server = "http://<host>"` | Matches containerd docs for insecure HTTP registry; aligns with existing CA file location |
| Global mirror http handling | Re-use existing `_default/hosts.toml` logic, preserving scheme | Minimal change; already formats URL with scheme, just need to ensure http is handled |
| Handler versioning | No version bump; fix is bug-fix for non-functional path, https outputs unchanged | Constitution VII requires version bump when patch output changes for working clusters; here only previously-broken http clusters will see new patches (desired fix), while working https clusters see no diff |
| Need detection | Extend `needContainerdConfiguration` / `needContainerdConfiguration` to also return true for http scheme | Ensures http imageRegistries without CA still trigger file generation |

## Out of Scope

- Changing API validation (already allows http)
- Adding `insecureSkipVerify` or TLS configuration for https with self-signed without CA
- Modifying kubelet credential provider logic for http (credentials handling already scheme-agnostic)
- Lifecycle handlers or other registry-related addons
