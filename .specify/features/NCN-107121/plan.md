<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Handler no-rollout invariant Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make "upgrading CAREN changes no existing Cluster" a CI-enforced invariant: golden output for every registered handler version, frozen per-version code and variable schemas, and an upgrade e2e test as a backstop.

**Architecture:** A request-level golden harness builds `GeneratePatches` requests from committed corpus inputs the same way the CAPI topology controller does, defaults variables with CAPI's public `webhooks.Cluster.DefaultAndValidateVariables`, calls each registered handler in-process against envtest, applies the returned JSON patches, and snapshots the patched templates. A shell check compares golden files, corpus inputs, version directories, and registered names against the last release tag. Handler code moves to `pkg/handlers/vN/` only, each with its own CRD YAML snapshot for variable discovery.

**Tech Stack:** Go, CAPI v1.13.6 (`exp/runtime/hooks/api/v1alpha1`, `webhooks`), controller-runtime envtest via `test/helpers`, `github.com/evanphx/json-patch/v5`, `sigs.k8s.io/yaml`, bash, golangci-lint depguard, CAPI e2e `ClusterctlUpgradeSpec`, GitHub Actions.

**Branch**: `NCN-107121-01-golden-harness` and the stack above it (see "Local-first delivery")
**Date**: 2026-10-27
**Spec**: [./spec.md](./spec.md)

## Global Constraints

- Every task that moves or copies handler code MUST leave all existing golden files byte-identical. `git status pkg/handlers/golden/testdata` must be clean after `make test.root GOTEST_RUN=TestGolden`.
- No new modules. `evanphx/json-patch/v5` and `sigs.k8s.io/yaml` are already in the root module graph; promoting them to direct dependencies goes through `make mod-tidy`.
- Prefer `devbox run --` for Go, make, and helm commands.
- Commit locally on the stack branches described in "Local-first delivery". Never push until the final local gate passes and the user approves. Commits must be signed (run with full permissions so the 1Password agent is reachable).
- Do not edit `.specify/memory/constitution.md` without explicit user approval (PR 10).
- Do not cut a CAREN release while PRs 1 to 6 are partially merged without first reading the PR 2 baseline report: if `v6` has drifted from `v0.50.0`, that release rolls `v6` Clusters.

## Summary

Today nothing enforces the invariant. `v6` imports almost every live mutator, tests only check that expected patch operations are present, variable discovery is unversioned while CAPI persists schema defaults into every Cluster, and there is no upgrade e2e test. This plan delivers enforcement first (PRs 1 to 3), then restructures to frozen per-version trees with the golden tests proving each move is output-neutral (PRs 4 to 7), then adds tooling, the e2e backstop, and docs (PRs 8 to 10).

## Technical Context

**Language/Version**: Go (see `go.mod`).
**Primary Dependencies**: `sigs.k8s.io/cluster-api` v1.13.6 (`v0.50.0` used v1.12.5; both export `webhooks.Cluster.DefaultAndValidateVariables`), `sigs.k8s.io/cluster-api/test` v1.13.6, controller-runtime envtest.
**Testing**: testify for the harness; envtest through `test/helpers.TestEnv` because `encryptionatrest`, `imageregistries/credentials`, `mirrors`, `httpproxy`, `coredns`, `controlplanevirtualip`, and `extraapiservercertsans` read or write the management cluster; bash self-check for the freeze script; Ginkgo for e2e.
**Target Platform**: CI (GitHub Actions) and local `make test`.
**Constraints**: harness must be portable to the `v0.50.0` tree for the baseline (PR 2), so it uses only CAREN handler interfaces and runtime hook request types, not CAPI's internal topology engine.
**Scale/Scope**: about 10.1k lines of non-test Go and 107 files per frozen version (26.8k lines including tests). 5 providers; 17 example Clusters seed the corpus.

### Why request-level, not CAPI `exp/topology/desiredstate`

`desiredstate.NewGenerator` would reproduce CAPI's engine exactly, but its output also changes when CAPI is bumped, it needs a fake runtime client, cluster cache, and blueprint construction, and it cannot run on `v0.50.0` (CAPI v1.12.5) without rework. Request-level output isolates what CAREN contributes. CAPI engine behavior is covered by the upgrade e2e test (PR 9).

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| I. API-First | Pass | No API type changes. CRD YAML is copied into `vN/`, not redefined. |
| II. Handler-per-Provider | Pass | Provider separation kept inside each `vN/`. Existing `v6` cross-provider import of `aws/mutation/cni/calico` from generic is preserved as-is in frozen copies. |
| III. Library-First | Pass | Registration moves to `pkg/handlers/versions.go`; `cmd/main.go` stays thin. |
| IV. Tests Required | Pass | Golden harness, coverage test, freeze-script self-check, upgrade e2e. |
| V. Code Style | Pass | Import aliases per `.golangci.yml`. |
| VI. Dependency Management | Pass | Two indirect dependencies become direct; no new modules. |
| VII. Handler Version Safety | Amend | This feature replaces the copy-on-bump procedure. Amendment in PR 10, needs approval. |
| VIII. Handler Documentation | Pass | Contributor docs in PR 10; no user-facing behavior change. |

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-107121/
├── spec.md
├── plan.md
└── baseline-v0.50.0.md        # PR 2 output: v6 drift report
```

### Source Code (target layout after PR 7)

```text
pkg/handlers/
├── versions.go                # AllHandlers(mgr): concatenates every vN.AllHandlers
├── removed-handlers.txt       # "<handler name> <last release that served it>"
├── golden/
│   ├── harness.go             # load corpus, build request, default, call, apply, serialize
│   ├── golden_test.go         # matrix over every registered handler
│   ├── coverage_test.go       # every mutator non-empty for >= 1 input
│   └── testdata/
│       ├── corpus/<provider>/<input>.yaml         # frozen inputs (multi-doc)
│       └── golden/<handler name>/<provider>/<input>.yaml
├── lifecycle/                 # unchanged, unversioned
├── options/ utils/            # shared, allowlisted for vN imports
├── v6/
│   ├── handlers.go            # AllHandlers(mgr) for v6; registers the unversioned discovery names
│   ├── variables/crds/*.yaml  # snapshot from v0.50.0 api/v1alpha1/crds
│   ├── variables/*.go         # discovery handlers
│   └── {aws,docker,eks,generic,nutanix}/mutation/...   # full frozen copy
└── v7/                        # same shape; open version; discovery names end in v7
hack/handlers/
├── seed-corpus.sh
├── open-version.sh
├── check-frozen.sh
├── check-frozen-selftest.sh
└── new-version.sh
make/handlers.mk
.github/workflows/handler-freeze.yml
.github/workflows/nightly-e2e.yml
test/e2e/upgrade_test.go
```

**Structure Decision:** Registration moves from `pkg/handlers/{provider}/handlers.go` to `pkg/handlers/vN/handlers.go`, aggregated by `pkg/handlers/versions.go`, so that cutting a version adds one import and one slice entry at a marker comment. `pkg/handlers/{aws,docker,eks,nutanix,generic}/` cease to exist.

## Key Design (read before coding)

### Request construction (mirrors CAPI)

Port holder references and builtin variables from CAPI v1.13.6 `internal/topology/patches/engine.go` (`createRequest`) and `internal/topology/variables` (builtins). One request per handler per input, containing every template of the ClusterClass, as CAPI sends to each external patch:

| Template | Holder kind | Holder field path |
| --- | --- | --- |
| `spec.infrastructure.templateRef` | Cluster | `spec.infrastructureRef` |
| `spec.controlPlane.templateRef` | Cluster | `spec.controlPlaneRef` |
| `spec.controlPlane.machineInfrastructure.templateRef` | control plane kind | `spec.machineTemplate.spec.infrastructureRef` |
| MD class `bootstrap.templateRef` | MachineDeployment | `spec.template.spec.bootstrap.configRef` |
| MD class `infrastructure.templateRef` | MachineDeployment | `spec.template.spec.infrastructureRef` |

Confirm every field path against the CAPI source during T1.3; the table is the expected result, not a substitute for reading the code.

### Variables and defaulting

1. Call the paired discovery handlers (cluster and worker vars for the provider and version) and build `ClusterClass.Status.Variables`.
2. Call `(&webhooks.Cluster{}).DefaultAndValidateVariables(ctx, cluster, nil, clusterClass)`.
3. If it returns errors, the golden file records the error list instead of patches. This makes validation tightening visible as a golden diff.
4. Request `Variables` = defaulted `spec.topology.variables` + `builtin`; MD items additionally get their MD overrides and MD builtins.

Pairing until PR 5: every patch handler pairs with the unversioned discovery names. After PR 5: `vN` patch handlers pair with `vN` discovery, and `v6` pairs with the unversioned names.

### Golden file content

YAML (`sigs.k8s.io/yaml`, sorted keys) of a list of `{holderRef, object}` for every item after applying the returned patches with `jsonpatch.DecodePatch` and `ApplyWithOptions(EnsurePathExistsOnAdd, AllowMissingPathOnRemove)`, the same options as `common/pkg/capi/clustertopology/patches/generator.go`. Items keep fixed template names from the corpus, so output is deterministic.

### Update semantics (FR-005)

`go test ./pkg/handlers/golden/ -update-golden=<vN>`:

- Writes golden files that do not exist yet, for any handler.
- Overwrites existing golden files only for handler names of version `<vN>`.
- `make handlers.golden-update` passes the open version from `hack/handlers/open-version.sh`.

### Version detection (`open-version.sh`)

- Last release: `git describe --tags --match 'v*' --abbrev=0`.
- Latest version N: highest `N` in names matching `[A-Za-z]+v([0-9]+)configpatch` in `pkg/handlers/`.
- Open if `git grep -q "v${N}configpatch" <last release> -- pkg/handlers` fails; prints `vN`, else prints nothing.

## Local-first delivery

All work is built and proven locally as a GitHub native stack, managed with `gh-stack` 0.3.0 from devbox. `gh stack <cmd>` below means `devbox run -- gh-stack <cmd>`. Nothing is pushed until the final local gate passes; then the whole stack is submitted in one go.

Only `gh stack submit`, `push`, `sync`, `link`, `merge`, and `unstack` (without `--local`) talk to GitHub. `init`, `add`, `modify`, `view`, and navigation are local; `rebase` only fetches. Do not run the remote commands before the user approves.

### Stack

Each branch is based on the previous one; on GitHub each PR's base is the branch below it, and the bottom PR's base is `main`. Every branch has its own worktree at `.worktrees/<branch>`.

- [x] Layer 00 `build/devbox-gh-stack` adds `gh-stack` to devbox and the `gh-stack` agent skill to `.agents/skills/`. nixpkgs only has 0.1.1, which cannot handle stack branches checked out in other worktrees, so 0.3.0 is built from `hack/flakes`.
- [x] `gh stack init --base main build/devbox-gh-stack`, then `gh stack add NCN-107121-01-golden-harness` from the top layer's worktree adopts the branch checked out in `.worktrees/NCN-107121-01-golden-harness`.
- [ ] Each later layer: from the top layer's worktree, `git worktree add -b NCN-107121-NN-<name> .worktrees/NCN-107121-NN-<name> HEAD`, then `gh stack add NCN-107121-NN-<name>`, then `direnv allow` in the new worktree.

| # | Branch | Contents (sections below) |
| --- | --- | --- |
| 00 | `build/devbox-gh-stack` | `gh-stack` in devbox |
| 01 | `NCN-107121-01-golden-harness` | Spec, plan, PR 1, PR 2 report |
| 02 | `NCN-107121-02-freeze-checks` | PR 3 |
| 03 | `NCN-107121-03-versioned-layout` | PR 4 |
| 04 | `NCN-107121-04-versioned-discovery` | PR 5 |
| 05 | `NCN-107121-05-align-v6` | PR 6 (dropped if the baseline shows no drift) |
| 06 | `NCN-107121-06-import-lint` | PR 7 |
| 07 | `NCN-107121-07-new-version` | PR 8 |
| 08 | `NCN-107121-08-upgrade-e2e` | PR 9 |
| 09 | `NCN-107121-09-docs` | PR 10 docs and rules (constitution amendment is a separate, later PR) |

### Keeping the stack current

- [ ] Before starting each branch and before the final gate: `gh stack rebase` (fetches `origin`, then rebases every layer onto the one below, from `main` up, across worktrees), then `git fetch origin --tags`. Every worktree must be clean first. Run with full permissions so rebased commits are re-signed.
- [ ] After every rebase, re-run the golden test on each branch. A golden diff after a rebase means a change on `main` altered handler output: port it into the right `vN` tree, and for anything touching `v6`, re-run the PR 2 baseline comparison.
- [ ] If `git describe --tags --match 'v*' --abbrev=0 origin/main` is newer than `v0.50.0`, STOP. Check whether that release serves `v7` names. If it does, `v7` is released: replan so that `v7` keeps the unversioned discovery schema of that release and the open version becomes `v8`.

### Final local gate (before pushing anything)

- [ ] Each branch green in isolation: in each layer's worktree, run `devbox run -- make test lint`. Each PR is squash-merged on its own, so every layer must pass alone.
- [ ] Top branch: `devbox run -- make pre-commit`; `git status` clean afterwards.
- [ ] `hack/handlers/check-frozen-selftest.sh` and `make handlers.check-frozen` pass; the latter also with a deliberately modified released file (fails) and with `FROZEN_HANDLER_OVERRIDE=true` (passes).
- [ ] `make handlers.golden-update` is idempotent: running it twice leaves `git status` clean.
- [ ] `make handlers.new-version` passes in a scratch clone (T8 success path) and refuses on the real tree.
- [ ] Upgrade e2e on Docker passes: `devbox run -- make e2e-test E2E_LABEL='provider:Docker' E2E_FOCUS='Upgrade'`.
- [ ] Negative upgrade e2e: a deliberately changed `v6` mutator makes the spec fail. Revert and confirm green again.
- [ ] Docker quick-start e2e still passes after the restructure: `devbox run -- make e2e-test E2E_LABEL='provider:Docker' E2E_FOCUS='Quick start' SKIP_BUILD=true`.
- [ ] `actionlint` clean for new and changed workflows.

What the gate cannot prove locally: that the workflows trigger as intended on GitHub (label events, `merge_group`, schedule) and that the label value reaches the job. These are verified on the first pushed PRs.

### Opening the PRs (after user approval)

- [ ] `gh stack view` to confirm order and that every layer is listed.
- [ ] `gh stack submit` pushes every branch, opens one draft PR per layer with the base chained to the layer below, and links them as a stack. Edit each PR title to `<type>: [NCN-107121] Title` and fill in `.github/pull_request_template.md`. Mark ready for review with `gh stack submit --open` when the drafts look right.
- [ ] CI runs on every layer: `checks.yml` triggers on `pull_request` without a base-branch filter. `handler-freeze` compares against the last release tag, so a layer's base branch does not affect it.
- [ ] Merge with squash from the bottom (`gh stack merge --squash` or the PR UI). GitHub re-signs commits when it rebases the remaining layers. After partial merges, run `gh stack sync` locally.
- [ ] Maintainer: create the `frozen-handler-override` label and make `handler-freeze` a required check once layer 02 merges.

## Delivery

Each section below is one stack branch and later one PR. Each must be independently mergeable and keep `main` green. PR titles follow `<type>: [NCN-107121] Title`.

---

### PR 1 — Golden harness and corpus (FR-001 to FR-005)

Title: `test: [NCN-107121] Add golden output tests for mutation handlers`

#### T1.1 — Seed the corpus

**Files:** Create `hack/handlers/seed-corpus.sh`, `pkg/handlers/golden/testdata/corpus/<provider>/<input>.yaml`.

- [x] Write `seed-corpus.sh`: for each example in `examples/capi-quick-start/` and each minimal Cluster in `hack/handlers/corpus-minimal/`, concatenates the provider's ClusterClass and templates from `charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses/<provider>-cluster-class.yaml` with the Cluster and the Secrets in the same file, substituting `${...}` placeholders with fixed values. Never overwrites an existing input.
- [x] Seed one input per example file (16), plus one minimal-variables input per provider (4).
- [x] Verify: `rg -l '\$\{' pkg/handlers/golden/testdata/corpus` returns nothing; every file contains exactly one `ClusterClass` and one `Cluster`.

**Acceptance:** 20 corpus files, no unresolved placeholders.

#### T1.2 — Corpus loader (TDD)

**Files:** Create `pkg/handlers/golden/harness.go`, `pkg/handlers/golden/harness_test.go`.

- [ ] Write a failing test: loading the Docker Cilium HelmAddon input returns one ClusterClass, one Cluster, the five referenced templates, and N fixtures.
- [ ] Run `devbox run -- go test ./pkg/handlers/golden/ -run TestLoad -count=1`; expect a compile failure.
- [ ] Implement `func LoadInput(path string) (*Input, error)`: split documents, decode with the scheme from `test/helpers`, index templates by kind and name.
- [ ] Re-run; expect PASS.

#### T1.3 — Request builder (TDD)

**Files:** Modify `harness.go`, `harness_test.go`.

- [ ] Write a failing test asserting holder kind, name, and field path for each item of the Docker input, and the `builtin` JSON for one MD item (`machineDeployment.class`, `.name`, `.topologyName`, `.version`) and the control plane item (`controlPlane.version`).
- [ ] Read CAPI v1.13.6 `internal/topology/patches/engine.go` and the builtin variable code; implement `func BuildRequest(in *Input, vars DefaultedVariables) *runtimehooksv1.GeneratePatchesRequest`.
- [ ] Re-run; expect PASS.

#### T1.4 — Defaulting (TDD)

**Files:** Modify `harness.go`, `harness_test.go`.

- [ ] Write a failing test: for the minimal Docker input, defaulted `workerConfig` contains `nodeRegistration.ignorePreflightErrors: [SystemVerification]` (a `+kubebuilder:default` in `nodeconfig_types.go`).
- [ ] Implement `func DefaultVariables(ctx, in *Input, discovery ...handlers.Named) (DefaultedVariables, field.ErrorList)` using `webhooks.Cluster.DefaultAndValidateVariables`.
- [ ] Add a test that an invalid value returns a non-empty error list.
- [ ] Re-run; expect PASS.

#### T1.5 — Execute, apply, serialize (TDD)

**Files:** Modify `harness.go`, `harness_test.go`; `go.mod` via `make mod-tidy`.

- [ ] Write a failing test: running a meta handler with a single fake mutator that sets one field produces a golden document whose object contains that field and whose other items are unchanged.
- [ ] Implement `func Run(ctx, c client.Client, h handlers.Named, in *Input, vars DefaultedVariables) ([]byte, error)`: create a namespace and the Cluster and fixtures in envtest, call `GeneratePatches`, apply patches, marshal YAML.
- [ ] Re-run; expect PASS. Run `devbox run -- make mod-tidy`; confirm only `require` promotions in `go.mod`.

#### T1.6 — Golden compare and update flag

**Files:** Create `pkg/handlers/golden/golden_test.go`.

- [ ] Implement the `-update-golden` flag with the update semantics above and a byte comparison that prints a unified diff on mismatch.
- [ ] Run each case twice and fail if the two outputs differ.
- [ ] Verify: with no golden files present, `-update-golden=v7` writes files for both `v6` and `v7`; a second run with `-update-golden=v7` after hand-editing a `v6` golden file does not overwrite it and the test fails.

#### T1.7 — Matrix over every registered handler

**Files:** Modify `golden_test.go`.

- [ ] Build a manager with `ctrl.NewManager(helpers.TestEnv.Config, ...)`; collect `AllHandlers(mgr)` from every provider package (later `pkg/handlers/versions.go`).
- [ ] For each handler implementing `GeneratePatches`, run every corpus input of the matching provider (generic handlers: every input); for each handler implementing `DiscoverVariables`, write one golden file of its response.
- [ ] Add a test that fails if a golden directory exists for a name that is not registered and not listed in `removed-handlers.txt` (file created in PR 3; treat a missing file as empty).
- [ ] Generate golden files: `devbox run -- make test.root GOTEST_RUN=TestGolden GOTEST_ARGS='-update-golden=v7'` (adjust to how `test.root` passes args).
- [ ] Verify: re-run without the flag passes; `git status` shows only new files.

#### T1.8 — Make targets and the NCN-117035 check

**Files:** Create `make/handlers.mk`, `hack/handlers/open-version.sh`; include `make/handlers.mk` from `Makefile`.

- [ ] Add `handlers.golden-update` (uses `open-version.sh`; exits non-zero if no open version).
- [ ] Verify SC-001 manually: remove `ntp.NewPatch()` (or the equivalent NTP mutator) from the `v6` generic list; `make test.root GOTEST_RUN=TestGolden` fails with a diff naming `genericClusterv6configpatch` or the provider handlers. Revert.

**Acceptance:** golden test green in `make test`; deliberate output change fails it.

---

### PR 2 — `v0.50.0` baseline report (evidence for FR-018, FR-019)

Not merged as code; produces `.specify/features/NCN-107121/baseline-v0.50.0.md` and baseline golden files kept for PR 6.

#### T2.1 — Run the harness on `v0.50.0`

- [ ] `git worktree add .worktrees/baseline-v0.50.0 v0.50.0`.
- [ ] Copy `pkg/handlers/golden/` (harness, tests, corpus) into it. Fix compile differences only (CAPI v1.12.5, handler names are `v6` current and `v5` previous there).
- [ ] Run with `-update-golden=v6`; copy the `v6` and unversioned discovery golden files to `/tmp/caren-baseline-v0.50.0/`.
- [ ] Inputs rejected by the `v0.50.0` schema produce error-list golden files; keep them.

#### T2.2 — Diff and report

- [ ] `diff -ru /tmp/caren-baseline-v0.50.0 pkg/handlers/golden/testdata/golden` limited to `v6` and discovery directories.
- [ ] Write `baseline-v0.50.0.md`: one section per differing file, the mutator responsible (bisect with `git log v0.50.0..origin/main -- <mutator dir>`), and whether the difference would roll Machines.
- [ ] STOP and review with the user if any difference exists. Each is a rollout for `v6` Clusters on the next release.

---

### PR 3 — Freeze checks (FR-006 to FR-009, FR-021)

Title: `ci: [NCN-107121] Reject changes to released handler output`

#### T3.1 — Removed-handlers record

**Files:** Create `pkg/handlers/removed-handlers.txt`.

- [ ] List the ten `v5` names (`{aws,docker,eks,generic,nutanix}{Cluster,Worker}v5configpatch`) with `v0.50.0`, verified with `git grep -ohE '"[A-Za-z]+v5configpatch"' v0.50.0 -- pkg/handlers | sort -u`.

#### T3.2 — Self-test for the freeze check (write first)

**Files:** Create `hack/handlers/check-frozen-selftest.sh`.

- [ ] Script builds a throwaway repo in `mktemp -d` with a `v1.0.0` tag containing a golden file, a corpus file, a `pkg/handlers/v1/` file, and a registered name; then asserts the exit status of `check-frozen.sh` for: no change (0), modified golden (1), deleted corpus file (1), modified `v1/` file (1), modified `v1/` file with `FROZEN_HANDLER_OVERRIDE=true` (0), modified golden with override (1), removed name (1), removed name listed in the record with its directories deleted (0), added golden file (0).
- [ ] Run it; expect failure because `check-frozen.sh` does not exist.

#### T3.3 — `check-frozen.sh`

**Files:** Create `hack/handlers/check-frozen.sh`; add `handlers.check-frozen` to `make/handlers.mk`.

- [ ] Golden and corpus files: `git diff --name-only --diff-filter=MD <tag> -- pkg/handlers/golden/testdata` excluding directories of names in the record; any output fails. Override does not apply.
- [ ] Version directories: for each `pkg/handlers/vN/` whose names exist at the tag and are not all in the record, `git diff --name-only --diff-filter=MD <tag> -- pkg/handlers/vN`; fail unless `FROZEN_HANDLER_OVERRIDE=true`.
- [ ] Registered names: names at the tag minus the record must be a subset of names at `HEAD`. Extract with `git grep -ohE '"[A-Za-z]+(v[0-9]+configpatch|ConfigVars(v[0-9]+)?)"'`. Mark with a `ponytail:` comment: regex extraction; upgrade path is a generated registered-names file.
- [ ] Run the self-test; expect PASS. Run `make handlers.check-frozen` on the branch; expect PASS.

#### T3.4 — Workflow

**Files:** Create `.github/workflows/handler-freeze.yml`.

- [ ] Trigger on `pull_request` types `[opened, synchronize, reopened, labeled, unlabeled]`, `merge_group`, and `push` to `main`. Separate workflow so label changes do not re-run all of `checks.yml`.
- [ ] `actions/checkout` with `fetch-depth: 0`; run self-test and `devbox run -- make handlers.check-frozen` with `FROZEN_HANDLER_OVERRIDE: ${{ contains(github.event.pull_request.labels.*.name, 'frozen-handler-override') }}`.
- [ ] Verify with `actionlint` (pre-commit).
- [ ] Manual, by a maintainer: create the `frozen-handler-override` label and make the workflow a required check.

---

### PR 4 — Versions-only layout (FR-010, FR-012, FR-020)

Title: `refactor: [NCN-107121] Move handler code into versioned trees`

Every task ends with: build, `make test.root GOTEST_RUN=TestGolden`, and `git status pkg/handlers/golden/testdata` clean.

#### T4.1 — Move live code to `v7`

- [ ] `git mv pkg/handlers/<p>/mutation pkg/handlers/v7/<p>/mutation` for each provider.
- [ ] Rewrite import paths (`rg -l` plus `sed` on `pkg/handlers/<p>/mutation` to `pkg/handlers/v7/<p>/mutation`), then `devbox run -- make fmt`.
- [ ] Verify golden files unchanged.

#### T4.2 — Per-version registration

**Files:** Create `pkg/handlers/v7/handlers.go`, `pkg/handlers/v6/handlers.go`, `pkg/handlers/versions.go`; modify `cmd/main.go`; delete `pkg/handlers/<p>/handlers.go`.

- [ ] `vN.AllHandlers(mgr)` returns that version's patch handlers and, for now, the unversioned discovery handlers are registered once from `versions.go`.
- [ ] `versions.go` contains a `// new-version:imports` and `// new-version:entries` marker for the script.
- [ ] Switch the golden matrix to `handlers.AllHandlers(mgr)`.
- [ ] Verify golden files unchanged and the set of registered names is identical (golden directory test).

#### T4.3 — Full `v6` copy

- [ ] Save `pkg/handlers/v6/generic/mutation/generic/imageregistries/credentials` aside; delete `pkg/handlers/v6`.
- [ ] `cp -R pkg/handlers/v7 pkg/handlers/v6`; rewrite imports `/v7/` to `/v6/` and names `v7configpatch` to `v6configpatch` inside `v6`.
- [ ] Restore the saved `credentials` package over the copy.
- [ ] Verify golden files unchanged for both versions. This proves the copy reproduces `main`'s `v6` output exactly.

#### T4.4 — Expose mutator lists and the coverage test (FR-004)

**Files:** Modify `pkg/handlers/v{6,7}/<p>/mutation/metapatch_handler.go`; create `pkg/handlers/golden/coverage_test.go`; add corpus inputs.

- [ ] Each provider exposes `ClusterMutators(mgr)` and `WorkerMutators(mgr)`; `MetaPatchHandler` and `MetaWorkerPatchHandler` use them. Verify golden files unchanged.
- [ ] Write the coverage test: for each version, provider, and mutator, wrap the mutator alone in `NewMetaGeneratePatchesHandler` and assert non-empty patches for at least one corpus input; name failures by `%T`.
- [ ] Add corpus inputs (new files, new golden files) until it passes.
- [ ] Verify: deleting the NTP input makes the coverage test fail.

#### T4.5 — Remove the dead hardcoded version

- [ ] In `hack/tools/clusterclass-v1beta2/main.go`, delete `deriveGeneratePatchesExtension` if no template under `hack/examples/overlays/clusterclasses/` lacks `generatePatchesExtension`; otherwise leave it for `new-version.sh`.
- [ ] Verify `devbox run -- make examples.sync` produces no diff.

---

### PR 5 — Versioned variable discovery (FR-013 to FR-015)

Title: `feat: [NCN-107121] Version variable discovery with handlers`

#### T5.1 — `v7` discovery from a CRD snapshot

**Files:** Create `pkg/handlers/v7/variables/crds/*.yaml`, `pkg/handlers/v7/variables/*.go`; move `pkg/handlers/<p>/{clusterconfig,workerconfig}` content into it; modify `make/go.mk` (`go-generate`).

- [ ] Write a failing test: `v7` discovery for AWS cluster returns a schema equal to `v1alpha1.AWSClusterConfig{}.VariableSchema()` and the handler name is `AWSClusterConfigVarsv7`.
- [ ] Copy `api/v1alpha1/crds/*.yaml`; `//go:embed` them; build schemas with `variables.MustSchemaFromCRDYAML`.
- [ ] `go-generate` refreshes the open version's snapshot after `controller-gen` (uses `open-version.sh`), so the open version tracks `api/` until release.
- [ ] Re-run; expect PASS. New discovery golden files appear for the `v7` names.

#### T5.2 — `v6` and the unversioned names from `v0.50.0`

- [ ] `git show v0.50.0:api/v1alpha1/crds/<file>` into `pkg/handlers/v6/variables/crds/` for every CRD file.
- [ ] `v6/variables` registers the existing unversioned names (`AWSClusterConfigVars`, and so on).
- [ ] Replace the unversioned discovery golden files with the PR 2 baseline. Verify they match.
- [ ] Switch harness pairing: `vN` patch handlers use `vN` discovery, `v6` uses the unversioned names.
- [ ] `v6` patch output can change here because its defaults now come from the `v0.50.0` schema. `v6` golden files are not yet released, so regenerate them with `-update-golden=v6` to keep `main` green, and list every changed file in the PR description. Compare them to the PR 2 baseline; remaining differences are the input to PR 6.

#### T5.3 — ClusterClass templates

- [ ] In `hack/examples/overlays/clusterclasses/*/kustomization.yaml.tmpl`, change `discoverVariablesExtension` to the `...configvarsv7-dv` names.
- [ ] `devbox run -- make examples.sync`; verify every default ClusterClass pairs `v7` patch and `v7` discovery names (`rg` both names per file).

---

### PR 6 — Align `v6` with `v0.50.0` (FR-018, FR-019)

Title: `fix: [NCN-107121] Restore v6 handler output to v0.50.0`

- [ ] Replace `v6` patch golden files with the PR 2 baseline.
- [ ] Run the golden test; for each failure, change only `pkg/handlers/v6/...` to restore `v0.50.0` behavior (use the bisected commit from the report). One commit per mutator.
- [ ] Verify golden test green; `v7` golden files untouched.
- [ ] If the PR 2 report showed no differences, this PR only replaces golden files (expected no-op) and can fold into PR 5.

---

### PR 7 — Import lint (FR-011)

Title: `ci: [NCN-107121] Forbid cross-version handler imports`

**Files:** Modify `.golangci.yml`.

- [ ] Add a depguard rule per version with `files: ["**/pkg/handlers/vN/**"]`, denying `.../pkg/handlers` and allowing `.../pkg/handlers/vN`, `.../pkg/handlers/utils`, `.../pkg/handlers/options`, `.../pkg/handlers/lifecycle/registry/utils`.
- [ ] Verify the rule fires: temporarily import a `v6` package from `v7`; `devbox run -- make lint` fails. Revert; lint passes.
- [ ] If depguard does not let a more specific allow override the deny, replace the rule with `pkg/handlers/imports_test.go` using `go list -deps -json` and record why in the PR.

---

### PR 8 — New-version tooling (FR-016, FR-017)

Title: `build: [NCN-107121] Add make target to cut a handler version`

**Files:** Create `hack/handlers/new-version.sh`; add `handlers.new-version` to `make/handlers.mk`.

- [ ] Refuse when `open-version.sh` prints a version: exit 1 naming it. Verify on this branch (`v7` is open).
- [ ] Steps: `cp -R vN v(N+1)`; rewrite `/vN/` imports and `vNconfigpatch`, `ConfigVarsvN` names; replace `variables/crds` with current `api/v1alpha1/crds`; add an import and entry at the `versions.go` markers; add a depguard rule; replace `vN` with `v(N+1)` names in `hack/examples/overlays/clusterclasses/*/kustomization.yaml.tmpl`; run `make examples.sync` and `make handlers.golden-update`.
- [ ] Verify the success path in a scratch clone: `git clone --local . /tmp/caren-newversion`, tag `HEAD` there as a fake release, run the target, then `make test` and `make lint` pass with no hand edits and `git status` shows only `pkg/handlers/v8/`, registration, lint config, templates, examples, and new golden files.

---

### PR 9 — Upgrade e2e (FR-022 to FR-024)

Title: `test: [NCN-107121] Add CAREN upgrade e2e with no-rollout check`

#### T9.1 — Upgrade-from flavor

**Files:** Modify `make/go.mk` (`e2e-test` prerequisites); add `test/e2e/data/upgrade-from-release/` to `.gitignore`.

- [ ] Target generates `cluster-template-upgrade-from-release.yaml` from `git show <last release>:charts/.../defaultclusterclasses/docker-cluster-class.yaml` (ClusterClass and templates renamed with an `-upgrade-from` suffix) and `git show <last release>:examples/capi-quick-start/docker-cluster-cilium-helm-addon.yaml` pointed at the renamed class.
- [ ] Verify the file references the last release's handler names (`v6configpatch` today).

#### T9.2 — e2e config

**Files:** Modify `test/e2e/config/caren.yaml`.

- [ ] Replace the unused `v0.49` entry with `${CAREN_UPGRADE_FROM_VERSION}` (exported by `make e2e-test` from `git describe`).
- [ ] Add the flavor file to the Docker provider `files:`.

#### T9.3 — Spec

**Files:** Create `test/e2e/upgrade_test.go`.

- [ ] `Describe("Upgrade", Label("provider:Docker"))` running `capie2e.ClusterctlUpgradeSpec` with `InitWithRuntimeExtensionProviders: caren:<last release>`, `Upgrades: [{RuntimeExtensionProviders: caren:v0.51.99}]`, `WorkloadFlavor: upgrade-from-release`, and current CAPI and CAPD versions for init and upgrade.
- [ ] `PreUpgrade`: record KubeadmControlPlane `spec`, MachineDeployment `spec`, and Cluster `spec.topology.variables`.
- [ ] `PostUpgrade`: set an annotation on the Cluster to trigger reconcile, wait for `TopologyReconciled=True` with `lastTransitionTime` after the annotation, then assert all three are deep-equal to the recorded values. The built-in `validateMachineRollout` covers Machines.
- [ ] Verify locally: `devbox run -- make e2e-test E2E_LABEL='provider:Docker' E2E_FOCUS='Upgrade'` passes.
- [ ] Verify the backstop once: change a `v6` mutator's output locally, rebuild, re-run; the spec fails. Revert.

#### T9.4 — Workflows

**Files:** Modify `.github/workflows/checks.yml`; create `.github/workflows/nightly-e2e.yml`.

- [ ] `checks.yml`: add `e2e-upgrade` calling `e2e.yml` with `provider: Docker`, `focus: Upgrade`, newest supported Kubernetes version. Confirm `focus: Quick start` and `focus: Self-hosted` do not match it.
- [ ] `nightly-e2e.yml`: `schedule` cron and `workflow_dispatch`, same job against `main`.
- [ ] Verify with `actionlint`.

---

### PR 10 — Docs, rules, constitution (FR-025, FR-026)

- [ ] Update `.cursor/rules/handler-version-safety.mdc`, `AGENTS.md`, `README.md` (remove v3 to v5 instructions), and add `docs/content/contributing/handler_versions.md`: layout, golden tests, `make handlers.golden-update`, `make handlers.new-version`, the override label, the removed-handlers record.
- [ ] Fix `docs/content/getting-started/integrating-with-your-clusterclass/_index.md` (wrong field name and missing `-gp`/`-dv` suffixes).
- [ ] Constitution principle VII amendment in its own PR. Ask the user before writing it.

## Spec Coverage

| Spec item | Task |
| --- | --- |
| FR-001, FR-003 | T1.5, T1.7 |
| FR-002 | T1.4, T5.2 |
| FR-004 | T1.1, T4.4 |
| FR-005 | T1.6, T1.8 |
| FR-006 to FR-009 | T3.2, T3.3, T3.4 |
| FR-010, FR-012 | T4.1 to T4.3, T5.1 |
| FR-011 | PR 7 |
| FR-013 to FR-015 | T5.1 to T5.3 |
| FR-016, FR-017 | PR 8 |
| FR-018, FR-019 | PR 2, PR 6 |
| FR-020 | T4.1 |
| FR-021 | T3.1 |
| FR-022 to FR-024 | PR 9 |
| FR-025, FR-026 | PR 10 |
| SC-001 | T1.8 |
| SC-002 | T5.1 (default added to `api/` changes only `v7` golden files) |
| SC-003 | T3.2 |
| SC-004 | T9.3 |
| SC-005 | PR 8 |
| SC-006 | T4.1 to T4.3, T5.1 |

## Risks

| Risk | Mitigation |
| --- | --- |
| Request builder diverges from CAPI's | T1.3 ports from source; upgrade e2e catches residual differences |
| `v0.50.0` harness port needs non-trivial changes | Harness uses only handler interfaces and hook request types; differences are confined to registration names |
| PR 2 finds drift | Stop and decide with the user before any release; PR 6 fixes inside `v6` only |
| Envtest makes the golden test slow | One envtest per package; namespaces per input; measure in T1.7 and parallelize subtests if needed |
| Upgrade e2e adds PR CI time | One Docker Kubernetes version on PRs; full matrix nightly if wanted later |

## Out of Scope

- Immutable or versioned default ClusterClasses in the chart.
- `failuredomainrollout` and `cluster-defaulter` update-path behavior.
- Deprecation process for handler versions.
- Automatic detection of when a new version is needed (NCN-112143).
