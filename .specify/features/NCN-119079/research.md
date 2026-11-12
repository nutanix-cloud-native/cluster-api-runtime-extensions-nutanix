<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Research: Support http schemes for container registries by properly configuring containerd

**Feature**: NCN-119079 | **Date**: 2026-11-11

## Research Questions

### RQ-01: How does containerd configure plain HTTP (insecure) registries?

**Decision**: Create per-registry file at `/etc/containerd/certs.d/<host>/hosts.toml` with:

```toml
server = "http://<host>"

[host."http://<host>"]
  capabilities = ["pull", "resolve"]
```

**Rationale**: According to https://github.com/containerd/containerd/blob/main/docs/hosts.md, the `certs.d` directory is used as `config_path` for registry configuration. Each host namespace (`/etc/containerd/certs.d/<registry-host>/hosts.toml`) can override the `server` URL. Setting `server = "http://<host>"` forces containerd to use plain HTTP instead of default HTTPS. Capabilities `pull` and `resolve` are required for image pulls.

**Alternatives Considered**:
- Configure via `_default/hosts.toml` for per-registry: rejected because `_default` is for default mirror that applies to all hosts, not per-host.
- Use `skip_verify = true`: rejected, that's for TLS verification skipping on https with self-signed, not plain http.
- Use containerd config.toml `insecure` flag (deprecated): rejected, deprecated since containerd 1.5, `config_path` is current method.

### RQ-02: Does global mirror already handle http?

**Decision**: Yes, existing `generateContainerdDefaultHostsFile` already preserves scheme via `formatURLForContainerd` (`%s://%s` + `/v2` path). For `http://10.111.45.34/library` it produces `[host."http://10.111.45.34/v2/library"]` in `_default/hosts.toml`. No change needed except confirming `needContainerdConfiguration` already true for Mirror=true.

**Rationale**: Mirror path is scheme-agnostic; tested with http mirror URL produces correct hosts.toml entry. Verified via unit test `Test_generateContainerdDefaultHostsFile_HTTP`.

**Alternatives**: No alternative; reuse existing.

### RQ-03: How to handle http imageRegistry without CA?

**Decision**: Extend `needContainerdConfiguration` to return true when `scheme == "http"`. Generate per-host hosts.toml even if no CA.

**Rationale**: Previously, imageRegistry without CA returned false, so no files were generated and http pulls failed. http requires explicit containerd configuration even without CA. This is the core bug-fix.

**Alternatives**: Keep existing logic and require CA for http – rejected, plain http registries often have no TLS and thus no CA.

### RQ-04: How to deduplicate same host with different paths?

**Decision**: Deduplicate per-host files by `URL.Host` (including port). Multiple http imageRegistries sharing same host produce single file at `/etc/containerd/certs.d/<host>/hosts.toml`. Path is ignored for per-host config because repository path is not part of host configuration; enabling http for host covers all paths under it.

**Rationale**: Registry identity is host:port, not path. Existing CA deduplication also keys on host only. For user example with `http://10.111.45.34/library` (mirror) and `http://10.111.45.34/app-images` (imageRegistry), they share host but use different files (`_default/hosts.toml` vs `10.111.45.34/hosts.toml`), so no deduplication conflict across mirror vs per-host.

**Alternatives**: Include path in HostURL – rejected, would create distinct host entries but same file path, requiring merging multiple entries in same file. Current single entry per host is simpler and covers all paths.

### RQ-05: CA handling for http?

**Decision**: Reuse existing `generateRegistryCACertFiles` which already handles CA for any scheme. For http with CA, produce both `ca.crt` file and per-host hosts.toml. Same host with different CA cert contents still errors via `ErrConflictingRegistryCACertificates`.

**Rationale**: CA file path is host-based, same as before. Conflict detection prevents ambiguous configuration.

## References

- containerd hosts docs: https://github.com/containerd/containerd/blob/main/docs/hosts.md
- Existing handler: `pkg/handlers/generic/mutation/generic/mirrors/containerd_files.go`
- API validation: `api/v1alpha1/clusterconfig_types.go` pattern `^https?://`
