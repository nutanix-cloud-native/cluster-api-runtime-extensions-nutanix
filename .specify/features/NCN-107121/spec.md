<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Enforce that upgrading CAREN never changes existing Clusters

**Jira Ticket**: [NCN-107121](https://jira.nutanix.com/browse/NCN-107121)
**Related**: [NCN-112143](https://jira.nutanix.com/browse/NCN-112143) (spike: automate handler versioning), [NCN-117035](https://jira.nutanix.com/browse/NCN-117035) (NTP rollout on upgrade), [NCN-117273](https://jira.nutanix.com/browse/NCN-117273) (unregistered handler)
**Feature Branch**: `NCN-107121-01-golden-harness` and the stack above it (see plan.md)
**Created**: 2026-10-27
**Status**: Draft
**Input**: User description: "enforce and validate that any changes to handlers do not result in any runtime changes for clusters, i.e. no rollouts or anything like that. it should be a strict invariant that upgrading CAREN should not affect existing clusters at all, only by opting in to new handler versions"

## Background

Constitution principle VII requires that upgrading CAREN is a no-op for existing Clusters. Nothing enforces it today:

- Previous handler versions are not frozen. `pkg/handlers/v6/` contains only wrapper files and one copied mutator; every other mutator is imported from the live `pkg/handlers/{provider}/mutation/` code, so changing a shared mutator changes `v6` output.
- `capitest.AssertGeneratePatches` matches with `ContainElements`, so extra patches pass. No test runs a full handler pipeline on fixed input and checks exact output, and nothing compares output across versions or releases.
- Variable discovery is unversioned. CAPI applies variable schema defaults to every Cluster on every topology reconcile and persists them, so adding or changing a `+kubebuilder:default` changes the variables of existing Clusters, and through them the patches of every handler version.
- Handler versions can be removed silently. `v0.50.0` shipped `v6` (current) and `v5` (previous); `main` removed `v5` in #1687. That removal is accepted, but nothing would have flagged it, and an unintended removal makes Clusters referencing the removed handler fail topology reconcile (the failure mode of NCN-117273).
- No end-to-end test upgrades CAREN under existing Clusters. `test/e2e/config/caren.yaml` declares an "upgrade from" version that nothing uses.

NCN-117035 is a production example: a frozen `v4` handler list was missing the NTP mutator, so clusters with NTP enabled rolled every Machine when CAREN was upgraded.

## Definitions

- **Handler version**: one numbered set of topology mutation handlers and variable discovery handlers, identified by `vN` in their registered names.
- **Released version**: a handler version whose handler names are registered in a `v*` release tag reachable from `HEAD`.
- **Open version**: the newest handler version, when it is not yet in any release. There is at most one open version.
- **Last release**: the most recent `v*` tag reachable from `HEAD` (`git describe --tags --match 'v*' --abbrev=0`).

## User Scenarios & Testing

### User Story 1 - CI fails when a released handler version's output changes (Priority: P1)

A contributor changes a mutator, a shared helper, an API type, a schema default, or a dependency. If that change alters what any released handler version produces for any input in the test corpus, CI fails before merge.

**Why this priority**: This is the enforcement mechanism. Every other story depends on being able to prove output is unchanged.

**Independent Test**: Reintroduce the NCN-117035 defect by removing one mutator from a released version's handler list. CI fails with a diff naming the handler, the input, and the changed object.

**Acceptance Scenarios**:

1. **Given** a released handler version, **When** a PR changes code that alters its patched output for any corpus input, **Then** the golden test fails and shows the diff.
2. **Given** a released handler version, **When** a PR adds or changes a schema default that alters its variables after defaulting, **Then** the golden test fails.
3. **Given** golden files that exist at the last release, **When** a PR modifies or deletes any of them, **Then** CI fails.
4. **Given** the open version, **When** a PR changes its output, **Then** the PR regenerates its golden files and CI passes.
5. **Given** a released version, **When** a PR adds a new corpus input and its golden file, **Then** CI passes.

---

### User Story 2 - Handler versions are fully frozen and explicit (Priority: P1)

All topology mutation and variable discovery code lives under `pkg/handlers/vN/`. There is no unversioned "current" tree. Once a version is released, its directory is frozen.

**Why this priority**: Shared code between versions is the main way released output changes today. Freezing removes the accidental path; User Story 1 catches what remains, such as API types and dependencies.

**Independent Test**: Grep for topology mutation handler registrations outside `pkg/handlers/vN/`; there are none. Modify any file under a released `vN/` without the override label; CI fails.

**Acceptance Scenarios**:

1. **Given** the repository, **When** listing packages that register topology mutation or variable discovery handlers, **Then** all are under `pkg/handlers/vN/`.
2. **Given** a package under `pkg/handlers/vN/`, **When** it imports another `pkg/handlers/vM/` package or any `pkg/handlers/` package outside its own version and an explicit allowlist, **Then** lint fails.
3. **Given** a released `vN/` directory, **When** a PR modifies or deletes a file in it without the override label, **Then** CI fails.
4. **Given** a released `vN/` directory, **When** a PR modifies a file in it with the `frozen-handler-override` label and every golden file is unchanged, **Then** CI passes. This is the path for fixes that do not change output, such as a panic on invalid input.

---

### User Story 3 - Variable schemas are versioned with their handlers (Priority: P1)

Each `vN/` contains a snapshot of the CRD YAML its variable schemas are built from and registers its own discovery handlers. Schema changes, including new defaults and tighter validation, reach only ClusterClasses that reference the new version.

**Why this priority**: CAPI persists schema defaults into existing Clusters on every reconcile, so unversioned schemas break the invariant even when patch code is frozen.

**Independent Test**: Add a `+kubebuilder:default` to an existing API field and regenerate. Discovery output for every released version is unchanged; only the open version's schema changes after its snapshot is refreshed.

**Acceptance Scenarios**:

1. **Given** a released version, **When** `api/` types or markers change, **Then** that version's discovered variable schema is unchanged.
2. **Given** existing ClusterClasses that reference the unversioned discovery names (for example `NutanixClusterConfigVars`), **When** CAREN is upgraded, **Then** those names remain registered and return exactly the schema shipped in the last release.
3. **Given** the default ClusterClass templates, **When** rendered, **Then** they reference the versioned discovery handlers for the same version as their patch handlers.

---

### User Story 4 - Cutting a new handler version is one command (Priority: P2)

A contributor who needs to change released output runs one make target. It creates `v(N+1)` from `vN`, rewrites import paths, bumps handler names, snapshots the current CRD YAML, registers the new version, and updates the ClusterClass templates.

**Why this priority**: Fully frozen copies are large (about 10k lines of non-test Go per version). Copying by hand is error-prone and is the cause of NCN-117035.

**Independent Test**: Run the target on a clean tree. `make test`, `make lint`, and `make examples.sync` succeed with no hand edits, and `git status` shows only the new version and the regenerated files.

**Acceptance Scenarios**:

1. **Given** the latest version is released, **When** the target runs, **Then** it creates the next version and the tree builds, lints, and tests clean.
2. **Given** an open version already exists, **When** the target runs, **Then** it exits non-zero and names the open version.

---

### User Story 5 - Upgrade end-to-end test proves no rollout (Priority: P2)

CI creates Clusters with the last release of CAREN, upgrades CAREN to the build under test, and asserts that nothing on those Clusters changed.

**Why this priority**: Backstop for anything the golden corpus does not cover, including CAPI behavior that unit tests cannot model.

**Independent Test**: Run the upgrade spec on the Docker provider. Deliberately break a released version's output; the spec fails.

**Acceptance Scenarios**:

1. **Given** Clusters created with the last release, **When** CAREN is upgraded and the topology controller has reconciled each Cluster at least once with the new CAREN, **Then** no Machine is replaced, KubeadmControlPlane and MachineDeployment `spec` are unchanged, and Cluster `spec.topology.variables` is unchanged.

---

### User Story 6 - Contributor guidance matches the enforcement (Priority: P3)

The constitution, the handler version safety rule, `AGENTS.md`, and `README.md` describe the versions-only layout, the golden tests, the override label, and the new-version target. Stale v3 to v5 instructions are removed.

**Why this priority**: Prevents contributors from following outdated manual steps.

**Independent Test**: Review each document against this spec.

**Acceptance Scenarios**:

1. **Given** the constitution amendment, **When** it is proposed, **Then** it goes in a dedicated PR with maintainer approval, per the constitution's governance section.

### Edge Cases

- **Dependency bumps** (CAPI, Kubernetes, JSON patch library) that change a released version's patched templates fail the golden test. The fix is a change inside the frozen version that restores identical output, made under the override label.
- **API Go type changes** that alter how a released version unmarshals variables fail the golden test, because frozen code still uses `api/v1alpha1` types.
- **Non-deterministic output**, such as map iteration order, must not exist. The golden test must produce identical output across repeated runs.
- **Handlers that read the management cluster**, such as registry credential Secrets, run against a fake client seeded from corpus fixtures.
- **Corpus gaps** let a change pass undetected. Every mutator in a version's handler list must produce non-empty output for at least one corpus input, so a mutator dropped from a list (NCN-117035) is always caught.
- **Removing a released version** deletes its golden files and fails CI. Removal requires an entry in the removed-handlers record and the override label; the deprecation process itself is out of scope.
- **Release branches**: the last release is resolved from `HEAD`, so a patch release branch compares against its own previous patch release.
- **The `v5` removal on `main`** is intentional. `v5` is recorded as removed so the registration check passes against `v0.50.0`, and it is not restored.
- **Chart-managed default ClusterClasses** are updated in place on `helm upgrade` and move every Cluster that uses them to the newest version. This breaks the invariant independently of this feature and is tracked separately. The upgrade e2e test must use ClusterClasses that the upgrade does not modify.

## Requirements

### Functional Requirements

#### Golden output tests

- **FR-001**: For every registered topology mutation handler (cluster and worker, every provider, every version), a test MUST run `GeneratePatches` over a fixed input corpus, apply the returned patches to the input templates, and compare the patched templates byte-for-byte against committed golden files.
- **FR-002**: Before calling a handler, the test MUST apply the default values from that handler version's variable schema to the input variables, in the same way as the CAPI topology controller.
- **FR-003**: For every registered variable discovery handler, a test MUST compare its `DiscoverVariables` output byte-for-byte against a committed golden file.
- **FR-004**: The corpus MUST include, per provider: a Cluster with only required variables, each generated example Cluster under `examples/`, and fixtures such that every mutator in each version's handler list produces non-empty output for at least one input. A mutator that cannot change any template of the default ClusterClasses on its own MAY be exempted, with the reason recorded next to the coverage test.
- **FR-005**: A make target MUST regenerate golden files for the open version only. It MUST NOT be able to rewrite golden files for released versions.

#### Immutability checks

- **FR-006**: CI MUST fail if any golden file or corpus input file that exists at the last release is modified or deleted. Corpus inputs are committed copies, not references to `examples/` or the chart, so that changing examples cannot change released golden output.
- **FR-007**: CI MUST fail if any file under a released `pkg/handlers/vN/` directory is modified or deleted, unless the PR has the `frozen-handler-override` label. Deletion of a version whose handler names are all in the removed-handlers record is not a failure.
- **FR-008**: The override label MUST NOT exempt a PR from FR-006.
- **FR-009**: CI MUST fail if any handler name registered in the last release is no longer registered, unless that name is listed in a committed removed-handlers record.

#### Versions-only layout

- **FR-010**: All topology mutation handlers, the mutators they compose, and variable discovery handlers MUST live under `pkg/handlers/vN/`. The unversioned `pkg/handlers/{aws,nutanix,docker,eks,generic}/mutation/` and `pkg/handlers/*/{clusterconfig,workerconfig}/` trees MUST NOT exist.
- **FR-011**: A lint rule MUST forbid any package under `pkg/handlers/vN/` from importing `pkg/handlers/vM/` (M != N) or any `pkg/handlers/` package outside an explicit allowlist.
- **FR-012**: Lifecycle hooks (`pkg/handlers/lifecycle/`) and webhooks remain unversioned.

#### Versioned variable discovery

- **FR-013**: Each `vN/` MUST contain the CRD YAML snapshot its variable schemas are built from, and MUST register discovery handlers whose names include `vN`.
- **FR-014**: The existing unversioned discovery handler names MUST remain registered and MUST return the schema shipped in the last release before this feature.
- **FR-015**: Default ClusterClass templates in `hack/examples/overlays/clusterclasses/` MUST reference patch and discovery handlers of the same version.

#### New-version tooling

- **FR-016**: A make target MUST create `v(N+1)` from the latest `vN`: copy, rewrite import paths, bump handler names, refresh the CRD YAML snapshot, register the new version, and update ClusterClass templates. After running it, `make examples.sync` MUST be the only other step needed.
- **FR-017**: The target MUST refuse to run when an open version already exists.

#### Migration

- **FR-018**: `v6` MUST become a fully frozen copy whose golden output equals the output of `v0.50.0`'s `v6` handlers on the corpus. The baseline MUST be generated by running the golden harness against `v0.50.0` code.
- **FR-019**: Any difference between `main` and `v0.50.0` for `v6` found by FR-018 MUST be resolved so that `v6` matches `v0.50.0`.
- **FR-020**: `v7` is the open version at the start of this work.
- **FR-021**: The `v5` handler names MUST be listed in the removed-handlers record. `v5` is not restored.

#### Upgrade end-to-end test

- **FR-022**: An e2e spec labelled for the Docker provider MUST install the last release of CAREN, create Clusters from that release's ClusterClasses, upgrade CAREN to the build under test, and assert that no Machines were replaced, using CAPI's `ClusterctlUpgradeSpec` machine-rollout validation.
- **FR-023**: The spec MUST also assert that KubeadmControlPlane `spec`, MachineDeployment `spec`, and Cluster `spec.topology.variables` are unchanged after the topology controller has reconciled each Cluster at least once with the new CAREN.
- **FR-024**: The spec MUST run in CI on every PR and on a nightly schedule against `main`.

#### Documentation

- **FR-025**: Update `.cursor/rules/handler-version-safety.mdc`, `AGENTS.md`, `README.md`, and `docs/content/` to describe the new layout, golden tests, override label, and new-version target.
- **FR-026**: Propose an amendment to constitution principle VII in a dedicated PR, subject to maintainer approval.

### Key Entities

- **Handler version (`vN`)**: Directory containing the mutators, meta patch handlers, discovery handlers, and CRD YAML snapshot for one version.
- **Golden corpus**: Input Clusters, templates, and management cluster fixtures shared by all versions.
- **Golden file**: Committed output for one (handler name, corpus input) pair: patched templates for mutation handlers, discovered schema for discovery handlers.
- **Last release tag**: Baseline for all immutability checks.
- **Removed-handlers record**: Committed list of handler names intentionally removed after release, each with the last release that served it.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Reintroducing the NCN-117035 defect (removing a mutator from a released version's handler list) fails CI.
- **SC-002**: Adding a `+kubebuilder:default` to an existing API field changes no released version's discovery or patch golden output.
- **SC-003**: Deleting a released handler version, or a single golden file, fails CI.
- **SC-004**: The upgrade e2e spec from the last release to the build replaces zero Machines and leaves Cluster variables unchanged.
- **SC-005**: Cutting a new version is one command, and the result passes `make test` and `make lint` with no hand edits.
- **SC-006**: No topology mutation or variable discovery handler code exists outside `pkg/handlers/vN/`.

## Design Decisions

| Decision | Choice |
| --- | --- |
| Primary enforcement | Golden output per registered handler, immutable after release |
| What is snapshotted | Patched templates after defaulting, not raw JSON patch operations |
| Freezing model | Full copy per version, no shared mutator code between versions |
| Variable schemas | Versioned with handlers via CRD YAML snapshot in `vN/` |
| Fixes to frozen code | Allowed with override label only when golden output is unchanged |
| Backstop | Upgrade e2e using CAPI `ClusterctlUpgradeSpec` on Docker |

## Out of Scope

- Making chart-managed default ClusterClasses immutable or versioned by name (separate ticket).
- Controllers and webhooks that can change existing Clusters: `failuredomainrollout` and the `cluster-defaulter` update path (separate ticket).
- Lifecycle hook and addon behavior on Kubernetes version upgrades.
- Automatically deciding when a new version is needed (NCN-112143). This spec makes that decision explicit and enforced instead.
- A deprecation and removal process for old handler versions.
