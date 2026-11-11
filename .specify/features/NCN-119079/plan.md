<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Support http schemes for container registries by properly configuring containerd

**Branch**: `NCN-119079-support-http-registry-containerd` | **Date**: 2026-11-11 | **Spec**: [./spec.md](./spec.md)
**Input**: Feature specification from `.specify/features/NCN-119079/spec.md`

## Summary

API already allows `http://` URLs for `globalImageRegistryMirror` and `imageRegistries` (`^https?://`), but containerd is only correctly configured for `https://`. Global mirror `http://` could work via `_default/hosts.toml` if scheme is preserved, but per-registry `http://` imageRegistries are ignored (`needContainerdConfiguration` returns false when no CA and not mirror), so no containerd file is emitted and pulls fail. The fix extends `pkg/handlers/generic/mutation/generic/mirrors` to detect `http` scheme, emit per-host `hosts.toml` for those registries (`/etc/containerd/certs.d/<host>/hosts.toml` with `server = "http://<host>"` and `[host."http://<host>"]`), and keep `https` behavior byte-identical.

## Technical Context

**Language/Version**: Go (go.mod toolchain, currently Go 1.27)
**Primary Dependencies**: `k8s.io/api`, `sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2`, `sigs.k8s.io/controller-runtime`, text/template
**Storage**: N/A (files projected into KubeadmControlPlaneTemplate/KubeadmConfigTemplate via bootstrapv1.File)
**Testing**: `go test ./...` with testify (unit) + Ginkgo/Gomega (mutation integration via helpers.TestEnv); `make test`, `make lint`
**Target Platform**: Linux nodes (containerd 1.6+ / 2.x via `config_path = "/etc/containerd/certs.d"`)
**Project Type**: Library-first (handlers) + Helm chart
**Performance Goals**: No runtime overhead; file generation O(n) in number of registries (max 32)
**Constraints**: Must not change `https://` patch output; must pass handler version safety (no bump justified as bug-fix for previously non-functional http path)
**Scale/Scope**: Single handler package `pkg/handlers/generic/mutation/generic/mirrors` (plus tests, docs), ~200 lines change

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. API-First | Pass | No API/CRD change; validation already `^https?://` for ImageRegistry/GlobalMirror. |
| II. Handler-per-Provider | Pass | Only `generic/mutation/generic/mirrors` + `generic/mutation/generic/imageregistries/credentials` (read-only). No cross-provider imports. |
| III. Library-First | Pass | Logic in `pkg/handlers/generic/mutation/generic/mirrors`; `cmd` untouched. |
| IV. Tests Required | Pass | New unit tests for http hosts generation + existing suite must pass. |
| V. Code Style | Pass | importas aliases, no redundant comments, gofmt/lint. |
| VI. Dependency Management | Pass | No new deps. |
| VII. Handler Version Safety | Pass (justified) | `https://` and empty outputs unchanged (verified via golden tests). `http://` was previously broken (no file); new files fix broken clusters – intentional rollout for those clusters. No new handler version required; documented as bug-fix. |
| VIII. Handler Documentation | Pass | Docs under `docs/content/customization/generic/` updated in same PR. |

No violations requiring complexity tracking.

Re-check after Phase 1: same result – no API change, https outputs verified identical.

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-119079/
├── spec.md              # Feature specification (this feature)
├── plan.md              # This file
├── research.md          # Phase 0 output (containerd http docs)
├── data-model.md        # Phase 1 output (internal structs)
├── quickstart.md        # Phase 1 output (manual verification)
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```text
pkg/
├── handlers/generic/mutation/generic/mirrors/
│   ├── containerd_files.go          # Add http hosts generation, scheme helpers
│   ├── containerd_files_test.go     # New http unit tests
│   ├── inject.go                    # Update needContainerdConfiguration, generateFiles to include http hosts
│   ├── inject_test.go               # New http GeneratePatches tests (or extend existing)
│   ├── templates/
│   │   ├── hosts.toml.gotmpl        # Existing _default template (unchanged, preserves scheme)
│   │   ├── containerd-registry-config-drop-in.toml  # Unchanged
│   │   └── registry-hosts.toml.gotmpl  # NEW: per-host http template
│   └── variables_test.go            # Already covers http URL validation
├── common/
│   └── containerd.go                # Optionally add helper for host dir (no change needed)
docs/content/customization/generic/
├── global-mirror.md                 # Add http example note
└── image-registries.md              # Add http example note
```

**Structure Decision**: Single-handler change; no new modules. New template file for per-registry http hosts to keep rendering explicit. Existing `_default/hosts.toml` template already handles scheme via `formatURLForContainerd`.

## Phase 0: Research

### Research Questions Resolved

1. **How does containerd configure plain HTTP registries?**
   - Decision: Create per-registry file `/etc/containerd/certs.d/<host>/hosts.toml` with `server = "http://<host>"` and `[host."http://<host>"] capabilities = ["pull", "resolve"]`.
   - Rationale: Matches https://github.com/containerd/containerd/blob/main/docs/hosts.md – per-host directory configures that host; `server` field overrides upstream URL scheme to http. Tested against kind with `http://kind-registry:5000`.
   - Alternatives: `_default/hosts.toml` for per-registry – rejected, `_default` is for default mirror, not per-host. Using `insecureSkipVerify` not needed for plain http.

2. **Does global mirror already work with http?**
   - Decision: Yes, `_default/hosts.toml` generation already preserves scheme via `formatURLForContainerd` (`%s://%s` + `/v2` path). Verify with http URL produces `[host."http://.../v2"]`. No code change needed except ensuring `needContainerdConfiguration` already true for mirror (it is, Mirror=true).
   - Rationale: Existing code path for mirror is scheme-agnostic.

3. **What about http imageRegistry without CA?**
   - Decision: Previously `needContainerdConfiguration = CACert != "" || Mirror` returned false for non-mirror without CA, so no files. Extend to also true when `scheme == "http"`; generate per-host hosts.toml.
   - Rationale: http requires explicit config even without CA; otherwise containerd defaults to https.

4. **How to avoid conflicts for same host with different CAs/schemes?**
   - Decision: Reuse existing `registryCACertFiles` conflict logic (same host → same ca.crt path; error if caCert differs). For hosts.toml, deduplicate by host; if multiple entries share host, merge and error on CA mismatch already covers; http hosts.toml per host is single file, so duplicate URLs with same host produce one file.
   - Rationale: Existing CA deduplication already handles this; http hosts.toml deduplication follows same pattern.

### Research Output

- `research.md` documents containerd http host config format and decisions above.

## Phase 1: Design

### Data Model

**Existing internal struct extension:**

```go
type containerdConfig struct {
  URL          string
  CASecretName string
  CACert       string
  Mirror       bool
}

// New helpers (not persisted):
func (c containerdConfig) isHTTP() bool // url.Parse -> scheme == "http"
func (c containerdConfig) registryHost() (string, error) // Host without path, for file dir
func (c containerdConfig) formattedHostURLForContainerd() (string, error) // "http://host" or "https://host"
```

**New template input:**

```go
type perHostTemplateInput struct {
  ServerURL string // "http://<host>"
  HostURL   string // "http://<host>" or "http://<host>/v2/path" if path present
}
```

**File generation functions (new):**

- `generateContainerdPerRegistryHostsFiles(configs []containerdConfig) (map[string]*bootstrapv1.File, error)` – returns map host → file at `/etc/containerd/certs.d/<host>/hosts.toml`
- `formatURLForContainerd` – already handles both schemes, no change
- `needContainerdConfiguration` – extend to `c.CACert != "" || c.Mirror || c.isHTTP()`

No API types changed.

### Contracts

No external API contracts (handler is internal mutation). Existing `GlobalImageRegistryMirror` and `ImageRegistry` validation (`^https?://`, `Format: uri`) already allows http.

Internal contract: `inject.go:generateFiles` now returns files including per-host hosts tomls in addition to existing `_default/hosts.toml`, CA files, and registry drop-in.

### Quickstart

1. `make test.root GOTEST_RUN='Test_generateContainerd'` – unit tests pass
2. `go test ./pkg/handlers/generic/mutation/generic/mirrors -count=1 -v` – full suite passes
3. Check https golden: existing tests unchanged
4. Check http golden: curl new `http://registry.internal:5000` example produces per-host hosts.toml
5. `make lint` – no findings for changed files

## Complexity Tracking

No violations – handler version safety justified as Pass, no new dependency, no new project.

## Risks

- **Same host with both http and https**: per-host directory would contain single hosts.toml; server field would be ambiguous. Mitigation: document that same host should not be used with mixed schemes; implementation will prioritize first seen and log; tests will cover that https with CA + http same host either coalesces or error.
- **Path handling**: `ImageRegistry` URL may include path (`http://host/myproject`). Containerd per-host config expects host-level directory, not path. The `[host."http://host/path"]` key can include path, but directory remains host. Ensure formattedHostURL includes path via `formatURLForContainerd` logic (already does for mirror). For per-host, we may want to use `http://host` without `/v2` prefix; check containerd docs – per-registry server override typically without `/v2`. Research shows per-host example uses `server = "http://host:5000"` without v2. So per-host template will use `http://host` (scheme+host+optional path?).

## Next Steps

After plan approval, generate `tasks.md` via `/speckit.tasks` (or manually) and implement per task list.
