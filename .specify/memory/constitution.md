<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# CAREN Constitution

## Core Principles

### I. API-First

All types and CRDs live in the `api/` module. Business logic never leaks into API definitions. API types are independently versioned and importable by consumers without pulling in the full runtime.

### II. Handler-per-Provider

Each infrastructure provider (Nutanix, AWS, Docker, EKS) gets its own isolated handler package under `pkg/handlers/`. No cross-provider imports. Shared logic belongs in `common/` or `pkg/handlers/generic/`.

### III. Library-First

Common logic lives in `common/` and `pkg/`. The `cmd/` package is a thin entrypoint that wires the controller-runtime manager, webhooks, and handlers together. It contains no business logic.

### IV. Tests Required

Every PR must include tests covering the changed behavior. No code merges without test coverage for new or modified functionality.

- **Unit tests**: Use `testify` (assert/require). One `_test.go` per package.
- **Integration tests**: Use Ginkgo/Gomega. Tag with `Integration` for `gotestsum` filtering.
- **E2E tests**: Use Ginkgo + CAPI test framework. Build-tagged `//go:build e2e`.

### V. Code Style

Automated linting via `golangci-lint` (35+ linters) and `pre-commit` is the baseline. Two additional rules:

- **Import aliases are mandatory**: Follow the conventions in `.golangci.yml` `importas` config (e.g. `clusterv1` for `sigs.k8s.io/cluster-api/api/v1beta1`).
- **No redundant comments**: Comments explain *why*, never narrate *what*. Obvious comments like `// create the client` or `// return the error` are not acceptable.

### VI. Dependency Management

- No new direct dependencies added to `go.mod` without explicit justification.
- Vendored/forked API types preferred over importing upstream provider packages directly.
- `depguard` linter config is the enforced blocklist — blocked packages must never be imported.

### VII. Handler Version Safety (No Silent Rollouts)

Upgrading CAREN on a management cluster MUST be a no-op for existing Clusters until they explicitly opt in. This is enforced through handler versioning:

- Every topology mutation handler name embeds a version (e.g. `awsClusterv5configpatch`).
- A **new handler version** MUST be created whenever:
  1. An existing handler's patch output changes in a way that would cause a Cluster or MachineDeployment rollout.
  2. A new handler is introduced that is enabled by default and produces patches affecting Cluster or Machine resources.
- When creating a new version (e.g. v5 -> v6):
  1. Copy the current implementation to `pkg/handlers/v{current}/`.
  2. Bump the version in the handler name (e.g. `v5` -> `v6`) in `pkg/handlers/{provider}/mutation/`.
  3. Register both old and new versions so existing ClusterClasses continue to work.
  4. Update default ClusterClass definitions to reference the new version.
- The old handler version MUST remain registered and functional indefinitely (until a documented deprecation cycle removes it).
- Violating this principle causes uncontrolled Machine rollouts across all managed clusters. There are no exceptions.

## Quality Gates

All of the following must pass before a PR can merge:

1. All CI checks green (lint, unit tests, e2e, vulnerability scan, conventional commit title).
2. At least one human code review approval.

No exceptions. Flaky e2e tests are fixed, not skipped.

## Versioning & Release

- **Semantic Versioning** (SemVer) for all releases.
- **Conventional Commits** enforced on PR titles — commit types drive automated version bumps.
- **release-please** automates version bumps, changelogs, and release PRs.
- **Goreleaser** builds binaries, container images (ko), and Helm chart bundles.

## Governance

This constitution supersedes ad-hoc practices. Amendments require a dedicated PR with at least one maintainer approval. The PR must document what changed and why.

**Version**: 1.1.0 | **Ratified**: 2026-02-28
