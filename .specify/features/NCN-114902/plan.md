<!--
Copyright 2026 Nutanix. All rights reserved.
SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Nutanix auth secret overlay selection for generated examples

**Branch**: `NCN-114902-nutanix-auth-secret-overlays`
**Date**: 2026-07-02
**Spec**: [./spec.md](./spec.md)

## Summary

Add two Nutanix example-generation overlay variants (`basic-auth`, `api-key`)
that patch only the `${CLUSTER_NAME}-pc-creds` JSON payload, and route
`hack/examples/sync.sh` through an auto-derived overlay selector. Keep non-Nutanix
generation paths unchanged and update docs/e2e variable metadata for API key input.

## Technical Context

**Language/Version**: Bash + YAML/Kustomize + Go project conventions  
**Primary Dependencies**: kustomize, existing `hack/examples` overlay pipeline  
**Storage**: N/A  
**Testing**: kustomize build verification + example sync script run  
**Target Platform**: local dev + CI example generation jobs  
**Project Type**: single Go repository with generated example artifacts  
**Performance Goals**: no meaningful change  
**Constraints**: preserve existing manifest shape outside of auth fields; prefer
inline JSON6902-style `patches` over deprecated patch forms  
**Scale/Scope**: overlay templates + sync script + docs/e2e config

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | No API/CRD changes |
| II. Handler-per-Provider | Pass | No handler package changes |
| III. Library-First | Pass | No new business logic in cmd |
| IV. Tests Required | Pass | Validate with focused generation checks |
| V. Code Style | Pass | Existing shell/yaml style preserved |
| VI. Dependency Management | Pass | No new deps |
| VII. Handler Version Safety | Pass | No mutation handler patch-output changes |
| VIII. Handler Documentation | Pass | User-facing docs updated for variable usage |

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-114902/
├── spec.md
└── plan.md
```

### Source Code (repository root)

```text
hack/examples/
├── overlays/clusters/
│   ├── nutanix/.../kustomization.yaml.tmpl
│   ├── nutanix-auth/basic-auth/.../kustomization.yaml.tmpl
│   └── nutanix-auth/api-key/.../kustomization.yaml.tmpl
├── patches/nutanix-auth/
│   ├── basic-auth/pc-credentials-secret.jsonpatch.yaml
│   └── api-key/pc-credentials-secret.jsonpatch.yaml
└── sync.sh

test/e2e/config/caren.yaml
docs/content/getting-started/create-cluster/nutanix.md
```

**Structure Decision**: Introduce auth-specific wrapper overlays that reuse existing
Nutanix overlays and apply a single targeted patch for `${CLUSTER_NAME}-pc-creds`.

## Tasks

### T1 — Add auth patch payloads

- Create JSON6902 patch files for basic-auth and api-key payloads
- Scope patch target to `Secret` `${CLUSTER_NAME}-pc-creds`

### T2 — Add auth wrapper overlays

- Add `nutanix-auth/basic-auth` and `nutanix-auth/api-key` overlay trees for each
  Nutanix cluster flavor (`cilium/calico`, `helm-addon/crs`, flow, failuredomains)
- Each wrapper references existing Nutanix overlay and adds the auth patch

### T3 — Route sync generation through selected auth overlay

- Update `hack/examples/sync.sh` to derive credentials overlay from env:
  `NUTANIX_CREDENTIALS_TYPE` if set, else from presence of `NUTANIX_API_KEY`,
  while preferring basic auth if `NUTANIX_USER` and `NUTANIX_PASSWORD` exist
- Build Nutanix examples from `nutanix-auth/<overlay>/...` paths

### T4 — Update user-facing variables/docs

- Add `NUTANIX_API_KEY` in `test/e2e/config/caren.yaml`
- Update Nutanix getting-started doc variable export examples and precedence note

### T5 — Verify

- Run targeted `kustomize build` on both auth overlay paths
- Run `hack/examples/sync.sh` and confirm generated Nutanix files reflect selected
  auth shape
- Run lint check for edited files

## Complexity Tracking

No constitution violations or exceptional complexity expected.
