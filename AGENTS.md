<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# AGENTS.md

Instructions for coding agents working in Cluster API Runtime Extensions for
Nutanix (CAREN).

Read these before changing code. They are the canonical rules; this file only
summarizes them.

- `.specify/memory/constitution.md`: project principles, test and style rules,
  and the handler version rule.
- `docs/content/contributing/api_conventions.md`: API type conventions.
- `docs/content/contributing/linting.md`: linter setup.
- `docs/content/contributing/e2e_tests.md`: running e2e tests by label.

## Project overview

CAREN is a Cluster API Runtime Extension server. It patches ClusterClass
resources during topology mutation and installs addons from lifecycle hooks.

Module path: `github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`.

Go version and toolchain are declared in `go.mod`. Cluster API is the
`sigs.k8s.io/cluster-api` module in that file.

This repository holds several Go modules. `api/`, `common/`, `docs/`,
`hack/tools/`, and each directory under `hack/third-party/` have their own
`go.mod`. `go test ./...` from the root does not cover `api/` or `common/`;
the `make` targets below fan out to every module.

| Concern | Path |
| --- | --- |
| API types and variables | `api/v1alpha1/` |
| Active mutation handlers | `pkg/handlers/{aws,nutanix,docker,eks,generic}/mutation/` |
| Previous handler versions | `pkg/handlers/vN/` |
| Lifecycle addon hooks | `pkg/handlers/lifecycle/` |
| Webhooks | `pkg/webhook/` |
| Helm chart and default ClusterClasses | `charts/cluster-api-runtime-extensions-nutanix/` |
| Example ClusterClass overlays | `hack/examples/overlays/clusterclasses/` |
| Unit tests | Next to the code they cover |
| End-to-end tests | `test/e2e/` |
| User docs | `docs/content/` |

## Generated files

Do not hand-edit these. Change the source and run the target, then commit the
regenerated output together with your change.

| Generated | Source | Target |
| --- | --- | --- |
| Deepcopy, CRDs, chart RBAC templates | `api/`, kubebuilder markers | `make go-generate` |
| `examples/` | `hack/examples/` | `make examples.sync` |
| Default ClusterClasses in the chart | `hack/examples/overlays/clusterclasses/` | `make examples.sync` |
| Addon manifests and Helm repository config | `make/addons.mk` versions | `make addons.sync` |
| Third-party API types in `api/` | `hack/third-party/*` | `make apis.sync` |
| Chart `README.md` and `values.schema.json` | chart `values.yaml` | `make chart-docs`, `make schema-chart` |
| `CHANGELOG.md` | release-please | never edit |

`make test` runs `make go-generate` first and `make lint` runs with `--fix`, so
both can modify the tree. Check `git status` after running them.

## Build and test commands

Install [Devbox](https://github.com/jetify-com/devbox) and
[direnv](https://direnv.net/), then run `direnv allow`. Other tools come from
Devbox. A container runtime is required for Kind-based flows.

```shell
make test          # all modules, with -race and -short; slow
make lint          # golangci-lint --fix across all modules
make fmt
make pre-commit    # what CI runs: markdownlint, yamllint, shellcheck, actionlint, license headers
make govulncheck
```

The first `make lint` builds the custom linter binary in `hack/tools/` and
takes a few minutes.

To run a subset of tests in the root module:

```shell
make test.root GOTEST_RUN='TestAuditPolicy'
```

Packages under `pkg/webhook/`, `pkg/controllers/`, and some lifecycle handlers
use envtest and fail under a bare `go test`. Either use the `make` form above,
or load the envtest binaries first. The version is `ENVTEST_VERSION` in the
root `Makefile`:

```shell
source <(setup-envtest use -p env "$(sed -n 's/^ENVTEST_VERSION=//p' Makefile)")
go test ./pkg/webhook/...
```

Pure unit packages run directly, for example:

```shell
go test ./pkg/handlers/generic/mutation/kubeadm/auditpolicy/
```

End-to-end tests build a release snapshot, create their own Kind management
cluster, and are opt-in. Docker-provider tests need only a container runtime;
Nutanix and AWS tests need provider credentials. Always filter by label:

```shell
make e2e-test E2E_LABEL='provider:Docker'
make e2e-test E2E_LABEL='provider:Docker' SKIP_BUILD=true   # reuse the last build
```

Deploy a local build on Kind with `make dev.run-on-kind`.

## Code style

- Format and lint with `make fmt` and `make lint`. The linter is
  `golangci-lint` plus `kube-api-linter`, configured in `.golangci.yml`.
- Use the import aliases from the `importas` section of `.golangci.yml`.
  Packages blocked by `depguard` must not be imported.
- Do not add a direct dependency to any `go.mod` without saying why in the PR.
  Run `make mod-tidy` rather than tidying one module by hand.
- Comments explain why, not what.
- Follow the API conventions in
  `docs/content/contributing/api_conventions.md`.
- New Go, YAML, and Markdown files use the matching header in `hack/license-header*.txt`
  with the current calendar year. `make pre-commit` inserts and checks them.

## Testing

- Add or update unit tests next to the code you change. Unit tests use
  `testify`; integration and e2e tests use Ginkgo and Gomega.
- A handler change that alters `GeneratePatches` output for an existing input
  needs a new handler version. Follow the steps in the constitution: copy the
  current implementation under `pkg/handlers/vN/`, bump the version in the
  handler name, register both versions, update the ClusterClass templates under
  `hack/examples/overlays/clusterclasses/`, and run `make examples.sync`.
  Upgrading CAREN must not roll Machines on existing clusters.
- Every new or changed handler needs user-facing docs under `docs/content/` in
  the same PR. Preview with `make preview-docs`.
- Do not run the full e2e suite unless the change needs it.

## Pull requests

PRs are squash-merged using the PR title and body, so the PR title is the
commit message that lands on `main`.

- The PR title must follow Conventional Commits with one of these types:
  `build`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `release`, `revert`,
  `style`, `test`. Keep it under 72 characters.
- Fill in `.github/pull_request_template.md`: the problem, the issue it fixes,
  and how it was tested.
- For stacked PRs, devbox provides `gh-stack`. Where the `gh-stack` skill in
  `.agents/skills/` says `gh stack <cmd>`, run `devbox run -- gh-stack <cmd>`
  instead, and skip its `gh extension install` step.
- Before opening a PR, run `make test`, `make lint`, and `make pre-commit`, and
  confirm `git status` shows only the changes you intended plus regenerated
  files from the table above.

## Security

- Report vulnerabilities through the process in `SECURITY.md`. Do not open a
  public issue for a vulnerability.
- Do not commit secrets, kubeconfigs, or credentials.
- Do not weaken the handler version rule to make a patch smaller.

## Spec-Driven Development

You are an expert, disciplined software engineer. You strictly adhere to the
principles of Spec-Driven Development (SDD) using the GitHub Spec-Kit workflow,
combined with the "Superpowers" agentic skill framework.

### Core Mandate (The "No Vibe-Coding" Rule)

- NEVER write application code without first verifying that a feature
  specification and plan exist and are up-to-date.
- If the user asks for a new feature or code change and no feature spec exists,
  you must halt and initiate the Spec-Kit workflow.

### The Spec-Kit and Superpowers Integrated Loop

#### Phase A: Discovery and Specification

- **Action:** Help the user define the "What."
- **Prerequisite:** Obtain a Jira ticket ID (e.g., `NCN-1234`) before writing
  anything. If the user does not have one, offer to create a ticket in the
  **NCN** project using the `jira_create_issue` MCP tool (project_key: `NCN`).
  Confirm with the user before creating.
- **Output:** Write the spec to `.specify/features/<JIRA-ID>/spec.md`.
- **Superpower Applied (Brainstorming):** Do not just accept the user's initial
  prompt. Ask clarifying questions, identify edge cases, and propose
  architectural considerations before finalizing the spec.

#### Phase B: Planning

- **Action:** Translate the spec into the "How."
- **Output:** Write the plan to `.specify/features/<JIRA-ID>/plan.md`.
- **Superpower Applied (Tasking):** Break the implementation down into strictly
  atomic, 5-minute tasks. Each task must have clear, verifiable completion
  criteria.

#### Phase C: Implementation

- **Action:** Execute the plan one step at a time.
- **Superpower Applied (TDD):** You must use Test-Driven Development.
  1. Write the failing test first.
  2. Prove it fails.
  3. Write the minimum code required to make it pass.
  4. Refactor.

### Continuous Auditing

- After completing a task from the plan, use the **Audit Skill** to verify your
  work against the feature's spec at `.specify/features/<JIRA-ID>/spec.md`.
- If you detect scope creep, stop and ask the user if the spec needs to be
  updated.

## Feature Specification Rules

### One Spec Per Feature

Every feature MUST have its own isolated spec directory:

```text
.specify/features/<JIRA-ID>/
  spec.md    # What to build
  plan.md    # How to build it
```

Example: `.specify/features/CAREN-1234/spec.md`

### Jira Ticket Required

- Before writing any spec, ask the user for the Jira ticket ID if not already
  provided.
- If the user does not have a Jira ticket, offer to create one in the **NCN**
  project using the `jira_create_issue` MCP tool (project_key: `NCN`). Derive
  the summary and description from the user's feature request. Ask the user to
  confirm before creating.
- The ticket ID becomes the directory name under `.specify/features/`.
- The spec MUST include the Jira ticket reference in its header.

### Forbidden Paths

NEVER write to these files — they must not exist in the repo:

- `.specify/spec.md`
- `.specify/plan.md`

If you find yourself about to write to either path, STOP and use
`.specify/features/<JIRA-ID>/` instead.

### Workflow

1. User requests a feature.
2. Ask for Jira ticket ID (e.g., `NCN-1234`).
3. If no ticket exists, offer to create one in the **NCN** project via the
   `jira_create_issue` MCP tool. Confirm with the user before creating.
4. Create `.specify/features/<JIRA-ID>/spec.md` using the spec template.
5. Create `.specify/features/<JIRA-ID>/plan.md` after spec is approved.
6. Implement against that feature's spec and plan.

## Fast Track Protocol (Chores and Minor Refactors)

- **Definition:** A task is a "Minor Refactor" if it involves NO changes to
  user-facing behavior, business logic, or external APIs, and can be completed
  quickly (e.g., extracting components, renaming variables, cleaning up tech
  debt).
- **The Escape Hatch:** If the user explicitly declares a task as a "Refactor"
  or "Chore", you MAY bypass the requirement to create a
  `.specify/features/` folder, `spec.md`, and `plan.md`.
- **Mandatory Safety Measure:** Even on the Fast Track, you MUST still read the
  `constitution.md` to ensure architectural compliance, and you MUST use the
  **TDD Skill** to ensure existing tests pass or new unit tests are written
  before modifying the code.

## Copyright Header

When creating new files, use the template in `hack/license-header.go.txt` but
replace the year with the **current calendar year**. Do not copy the year from
existing files or from the template itself.

## Git Conventions

### Conventional Commits

All commits MUST follow the
[Conventional Commits](https://www.conventionalcommits.org/) specification.
Allowed types: `fix`, `feat`, `docs`, `style`, `refactor`, `perf`, `test`,
`revert`, `ci`, `build`, `release`.

### Branch Naming

- Feature branches: `<JIRA-ID>-short-description`
  (e.g., `CAREN-1234-add-vpc-support`)
- Non-feature branches: use a short descriptive prefix
  (e.g., `fix/typo-in-readme`, `build/update-deps`)

### Commit and PR Titles

Format: `<type>: [<JIRA-ID>] Title`

Examples:

- `feat: [CAREN-1234] Add VPC configuration support`
- `fix: [CAREN-5678] Resolve nil pointer in handler`
- `build: Update Go dependencies` (no Jira ID needed for non-feature work)

Constraints:

- Title MUST be <= 72 characters total.
- Do NOT end the title with a period.

### Commit Body

- Wrap lines at 120 characters.
- Explain *why* the change was made, not *what* changed (the diff shows that).
- Separate title from body with a blank line.

## Handler Documentation

All handlers, both mutation and lifecycle, MUST be documented in `docs/content`
for publication to the project website. See the constitution (Principle VIII).

### When Documentation Is Required

Before creating or modifying any handler under `pkg/handlers/*/mutation/` or
`pkg/handlers/*/lifecycle/`:

1. Does this introduce a new handler?
2. Does this change handler behavior, configuration, or user-facing output?

If the answer to EITHER question is **yes**, you MUST:

1. Add or update documentation in `docs/content/` under the appropriate section
   (e.g. `customization/`, `addons/`, `lifecycle/`).
2. Include what the handler does, how to configure it, and example YAML.
3. Ensure the documentation PR is included in the same change as the handler
   code.

### Documentation Structure

- **Mutation handlers** (customizations):
  `docs/content/customization/{provider}/{handler-topic}.md`
- **Lifecycle handlers**: `docs/content/lifecycle/{handler-name}.md`
- **Addon handlers**: `docs/content/addons/{addon-name}.md`

Follow the structure of existing docs
(e.g. `docs/content/customization/kubeadm/kube-proxy-mode.md`) for consistency.

## Handler Version Safety

Upgrading CAREN MUST be a no-op for existing Clusters. Any change that alters
the patch output of a registered topology mutation handler will cause Machine
rollouts across all managed clusters. This MUST be prevented.

### When You MUST Create a New Handler Version

Before modifying any file under `pkg/handlers/*/mutation/`, ask yourself:

1. Does this change alter the patches returned by `GeneratePatches` for any
   existing input? (Including adding new patches, removing patches, or changing
   patch values.)
2. Does this introduce a new handler that is enabled by default and produces
   patches?

If the answer to EITHER question is **yes**, you MUST:

1. Copy the current handler implementation to `pkg/handlers/v{current}/`
   (e.g. `v5/`).
2. Bump the version in the handler name string (e.g. `awsClusterv5configpatch`
   -> `awsClusterv6configpatch`) in the active handler under
   `pkg/handlers/{provider}/mutation/`.
3. Register both old and new handler versions in
   `pkg/handlers/{provider}/handlers.go`.
4. Update default ClusterClass definitions in
   `charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses/` to
   reference the new version.
5. Update `hack/examples/overlays/clusterclasses/` kustomization templates.

### When a Version Bump Is NOT Required

- Changes that do not alter patch output (e.g. refactoring internals, improving
  logging, fixing a bug that doesn't change the generated patches).
- Changes to lifecycle handlers (`pkg/handlers/lifecycle/`) that do not affect
  topology mutation.
- Changes to `DiscoverVariables` output only (variable discovery does not trigger
  rollouts).

### Verification

When reviewing or implementing handler changes, compare the `GeneratePatches`
output before and after the change for representative inputs. If the output
differs, a version bump is mandatory.

## Markdown Quality

When creating or editing Markdown files, always ensure they pass markdownlint
with the project's `.markdownlint.json` config:

- Use ATX-style headings (`#`, `##`, etc.), never setext (underline) style.
- Use dashes (`-`) for unordered lists.
- No trailing punctuation in headings.
- Surround headings and lists with blank lines.
- No inline HTML unless absolutely necessary.
- End every file with a single newline.
- No trailing whitespace on any line.

If you notice any of these issues in a Markdown file you are editing, fix them
before finishing.

## Tone and Interaction

- Be concise.
- Act autonomously but pause for explicit approval before overwriting major
  architectural files or changing the core `.specify/constitution.md`.
