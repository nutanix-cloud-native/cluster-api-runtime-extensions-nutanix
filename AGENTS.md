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
- Before opening a PR, run `make test`, `make lint`, and `make pre-commit`, and
  confirm `git status` shows only the changes you intended plus regenerated
  files from the table above.

## Security

- Report vulnerabilities through the process in `SECURITY.md`. Do not open a
  public issue for a vulnerability.
- Do not commit secrets, kubeconfigs, or credentials.
- Do not weaken the handler version rule to make a patch smaller.
