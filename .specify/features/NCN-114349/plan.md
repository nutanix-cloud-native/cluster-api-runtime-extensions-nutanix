<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Privileged PSA label on ntnx-system and node-feature-discovery namespaces

**Branch**: `NCN-114349-psa-label-ntnx-system-nfd-namespaces`
**Date**: 2026-05-14
**Spec**: [./spec.md](./spec.md)

## Summary

Three lifecycle handlers must ensure a workload-cluster namespace exists with
`pod-security.kubernetes.io/enforce=privileged` *before* installing their
Helm-based addons. The required behaviour and helper already exist in
`pkg/handlers/utils.EnsureNamespaceWithMetadata`, and the MetalLB lifecycle
handler is the canonical template
(`pkg/handlers/lifecycle/serviceloadbalancer/metallb/handler.go`). We follow
that template in each of the three affected handlers and add a tiny shared
constant for the label map. No topology mutation handlers are touched, so no
version bump is required.

## Technical Context

**Language/Version**: Go (matches `go.mod`).
**Primary Dependencies**: controller-runtime, sigs.k8s.io/cluster-api, in-repo
`pkg/handlers/utils`.
**Testing**: A single Ginkgo integration test in
`pkg/handlers/utils/secrets_integration_test.go` (the same file that already
covers `EnsureSecretOnRemoteCluster` with `helpers.TestEnv.WithFakeRemoteClusterClient`)
extended to prove that `EnsureNamespaceWithMetadata` on the remote cluster
applies the privileged PSA label idempotently. The three call-site changes
in the handler files are simple, one-line glue and rely on this single test
plus existing reviewer scrutiny — matching the precedent set by the MetalLB
handler.
**Target Platform**: management cluster controllers; workload cluster API as
the remote target.
**Project Type**: Go module / Kubernetes controller (single project).
**Performance Goals**: N/A — single SSA call per Apply.
**Constraints**: server-side apply, must be idempotent across multiple
handlers ensuring the same namespace, must not change handler patch output.
**Scale/Scope**: 3 handler files + 1 shared constant + tests + docs.

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | No API/CRD changes. |
| II. Handler-per-Provider | Pass | Each handler does its own ensure; shared constant lives in `pkg/handlers/utils`. |
| III. Library-First | Pass | New constant is added to existing `pkg/handlers/utils`. |
| IV. Tests Required | Pass | Each handler change ships with a unit test covering the namespace ensure. |
| V. Code Style | Pass | No new comments narrating "what". Import aliases follow existing files. |
| VI. Dependency Management | Pass | No new deps. |
| VII. Handler Version Safety (No Silent Rollouts) | Pass | Lifecycle handlers only; no `GeneratePatches` change. Verified by inspection — no files under `pkg/handlers/*/mutation/` are modified. |
| VIII. Handler Documentation | Pass | Docs updated under `docs/content/addons/` for the three addons. |

No violations.

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-114349/
├── plan.md                # This file
└── spec.md                # Feature specification
```

### Source Code (repository root)

```text
pkg/
├── handlers/
│   ├── utils/
│   │   └── utils.go                # Add PrivilegedPodSecurityLabels var
│   └── lifecycle/
│       ├── csi/nutanix/
│       │   ├── handler.go          # Ensure ntnx-system with PSA label
│       │   └── handler_test.go     # New unit test
│       ├── konnectoragent/
│       │   ├── handler.go          # Ensure ntnx-system with PSA label
│       │   └── handler_test.go     # New unit test
│       └── nfd/
│           ├── handler.go          # Ensure NFD ns with PSA label (Helm only)
│           └── handler_test.go     # New unit test

docs/
└── content/addons/
    ├── nfd.md                      # Mention PSA labelling
    ├── konnector-agent.md          # Mention PSA labelling
    └── (csi page)                  # Mention PSA labelling
```

**Structure Decision**: Stay with the existing single-project layout. Each
affected handler gets one small additive change plus a focused unit test.
Share the label-map constant from `pkg/handlers/utils` to avoid drift.

## Tasks

Atomic, TDD-ordered. Each is independently verifiable; do not merge a task
until its acceptance check passes.

### T1 — Shared constant

**Files**: `pkg/handlers/utils/utils.go`

Add an exported variable:

```go
// PrivilegedPodSecurityEnforceLabels is the label set applied to addon
// namespaces on workload clusters that contain workloads requiring the
// privileged Pod Security Standard (hostPath, hostNetwork, privileged
// containers, etc.). The PSA version label is intentionally omitted because
// "latest" is the PSA default and is the value we want.
var PrivilegedPodSecurityEnforceLabels = map[string]string{
    "pod-security.kubernetes.io/enforce": "privileged",
}
```

**Acceptance**: `go build ./...` succeeds; no consumers yet.

### T2 — Integration test for the underlying primitive (TDD)

**File**: `pkg/handlers/utils/secrets_integration_test.go` (extend)

Add a new Ginkgo `It` block:

- Stand up a `Cluster` and `WithFakeRemoteClusterClient` like the existing tests.
- Call `EnsureNamespaceWithMetadata(ctx, remoteClient, "test-ns",
  PrivilegedPodSecurityEnforceLabels, nil)`.
- Get the namespace on the remote client and assert
  `metadata.labels["pod-security.kubernetes.io/enforce"] == "privileged"` and
  that no `enforce-version` label was set.
- Call `EnsureNamespaceWithMetadata` again with the same arguments and
  reconfirm — idempotency.
- Pre-create the namespace without labels, call
  `EnsureNamespaceWithMetadata`, and confirm the label was added without
  conflict.

**Acceptance**: New `It` blocks pass under
`make test-integration` / the integration-tagged test run for
`pkg/handlers/utils`.

### T3 — Nutanix CSI handler

**File**: `pkg/handlers/lifecycle/csi/nutanix/handler.go`

In `Apply`, after the existing call to `CopySecretToRemoteCluster` succeeds
(or, more robustly, before `strategy.Apply`), obtain a remote-cluster client
via `remote.NewClusterClient` and call
`handlersutils.EnsureNamespaceWithMetadata(ctx, remoteClient,
defaultHelmReleaseNamespace, handlersutils.PrivilegedPodSecurityEnforceLabels,
nil)`. Mirror the MetalLB pattern exactly.

**Acceptance**: Package still builds and `make test` for the package is
green.

### T4 — Konnector Agent handler

**File**: `pkg/handlers/lifecycle/konnectoragent/handler.go`

In `apply`, after the existing `CopySecretToRemoteCluster` call (which already
constructs a remote client internally but doesn't expose it), build the
remote client and call `EnsureNamespaceWithMetadata` exactly as in T3 for
`defaultHelmReleaseNamespace` (= `ntnx-system`).

**Acceptance**: Package builds; package tests pass.

### T5 — NFD handler, Helm strategy only

**File**: `pkg/handlers/lifecycle/nfd/handler.go`

In the `case v1alpha1.AddonStrategyHelmAddon:` branch only — before
`strategy.Apply` — build a remote client and call
`EnsureNamespaceWithMetadata(ctx, remoteClient, defaultHelmReleaseNamespace,
handlersutils.PrivilegedPodSecurityEnforceLabels, nil)`. The CRS branch is
left untouched because the CRS ConfigMap already labels the namespace.

**Acceptance**: Package builds; package tests pass.

### T6 — Verify no handler-version bump needed

Run `git diff origin/main -- pkg/handlers` and confirm no files under
`pkg/handlers/*/mutation/` are modified. Per
`.cursor/rules/handler-version-safety.mdc`, lifecycle handler changes that
don't alter mutation patches do not require a version bump.

**Acceptance**: `find pkg/handlers -path '*/mutation/*' -newer .specify/features/NCN-114349/plan.md` lists nothing material.

### T7 — Docs

**Files**:

- `docs/content/addons/nfd.md`
- `docs/content/addons/konnector-agent.md`
- the CSI addon docs page (look up under `docs/content/addons/`)

Add a short paragraph under each addon explaining that CAREN labels the
addon's workload-cluster namespace with
`pod-security.kubernetes.io/enforce=privileged`, why (workloads need
privileged pod features), and that this happens automatically on every
reconcile.

**Acceptance**: Markdown files lint clean (per
`.cursor/rules/markdown-quality.mdc`); the Dirty Frag doc still reads
correctly (no contradiction).

### T8 — Lint + full test pass

Run repo-standard linters and the affected test packages.

**Acceptance**:

```bash
golangci-lint run ./pkg/handlers/...
gotestsum --format pkgname -- ./pkg/handlers/lifecycle/csi/nutanix/... \
                              ./pkg/handlers/lifecycle/konnectoragent/... \
                              ./pkg/handlers/lifecycle/nfd/...
```

both pass.

### T9 — Manual smoke verification (optional, before commit)

Build the operator image, deploy to a kind cluster with a Nutanix-flavoured
workload cluster, set `clusterConfig.podSecurityAdmission.enforce: baseline`,
and confirm `ntnx-system` and `node-feature-discovery` pods stay healthy and
the namespaces carry the expected label.

**Acceptance**: `kubectl get ns ntnx-system node-feature-discovery
-o jsonpath='{range .items[*]}{.metadata.name}{": "}{.metadata.labels.pod-security\.kubernetes\.io/enforce}{"\n"}{end}'`
prints `privileged` for each.

## Complexity Tracking

No constitution violations. No complexity worth tracking.
