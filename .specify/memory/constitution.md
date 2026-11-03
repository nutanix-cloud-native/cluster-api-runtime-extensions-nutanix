<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# CAREN Constitution

## Core Principles

### I. API-First

All types and CRDs live in the `api/` module. Business logic never leaks into API definitions. API types are independently versioned and importable by consumers without pulling in the full runtime.

### II. Handler-per-Provider

Each infrastructure provider (Nutanix, AWS, Docker, EKS) gets its own isolated handler package under `pkg/handlers/vN/`. No cross-provider or cross-version imports. Shared logic belongs in `common/` or `pkg/handlers/vN/generic/`.

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

Upgrading CAREN on a management cluster MUST be a no-op for existing Clusters until they opt in by naming a new handler version in their ClusterClass.

- All topology mutation handlers, the mutators they compose, and variable discovery handlers live under `pkg/handlers/vN/`, and their names embed the version (e.g. `awsClusterv7configpatch`, `AWSClusterConfigVarsv7`). Each version builds its variable schemas from its own CRD snapshot.
- The highest version is open until it is released. Released versions are frozen: their code, golden output, golden test inputs and handler names MUST NOT change.
- Any change to what a handler returns (patches, variable schemas or defaults) goes into the open version. When no version is open, `make handlers.new-version` cuts one.
- Golden tests cover every registered handler. `make handlers.check-frozen` runs in CI against the last release. An upgrade e2e test runs on every PR and nightly and fails on any topology change or Machine rollout.
- The `frozen-handler-override` label permits changes to released handler code that leave its golden output unchanged. Nothing permits changes to released golden output or handler names.
- A released handler name remains registered until a documented deprecation lists it in `pkg/handlers/removed-handlers.txt`.
- Violating this principle causes uncontrolled Machine rollouts across all managed clusters. There are no exceptions.

### VIII. Handler Documentation

All handlers, both mutation and lifecycle, MUST be documented in `docs/content` for publication to the project website. Every handler under `pkg/handlers/vN/*/mutation/` and `pkg/handlers/lifecycle/` must have corresponding user-facing documentation that explains what the handler does, how to configure it, and includes example YAML. New or modified handlers require documentation updates in the same PR.

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

**Version**: 1.2.0 | **Ratified**: 2026-02-28 | **Last Amended**: 2026-11-03
