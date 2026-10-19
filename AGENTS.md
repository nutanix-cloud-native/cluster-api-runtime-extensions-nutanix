<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# AGENTS.md

Instructions for coding agents working in Cluster API Runtime Extensions for
Nutanix (CAREN).

## Project overview

CAREN is a Cluster API Runtime Extension server. It patches ClusterClass
resources during topology mutation and installs addons from lifecycle hooks.

Module path: `github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`.

Go version and toolchain are declared in `go.mod`. Cluster API is the
`sigs.k8s.io/cluster-api` module in that file.

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

## Build and test commands

Install [Devbox](https://github.com/jetify-com/devbox) and
[direnv](https://direnv.net/), then run `direnv allow`. Other tools come from
Devbox. A container runtime is required for Kind-based flows.

```shell
make test
make lint
make fmt
make govulncheck
```

Run one package's tests with the Go toolchain from Devbox, for example:

```shell
go test ./pkg/handlers/generic/mutation/kubeadm/auditpolicy/
```

End-to-end tests need a cluster and are opt-in:

```shell
make e2e-test
```

Deploy a local build on Kind with `make dev.run-on-kind`.

## Code style

- Format and lint with `make fmt` and `make lint`. The linter is
  `golangci-lint` plus `kube-api-linter`, configured in `.golangci.yml`.
- Follow the API conventions in
  `docs/content/contributing/api_conventions.md`.
- New Go, YAML, and Markdown files use the matching header in `hack/license-header*.txt`
  with the current calendar year.
- Commit titles follow Conventional Commits and stay within 72 characters.
  Explain why in the body.

## Testing

- Add or update unit tests next to the code you change.
- A handler change that alters `GeneratePatches` output for an existing input
  needs a new handler version. Copy the current implementation under
  `pkg/handlers/vN/`, bump the handler name, register both versions, and update
  the default ClusterClasses and `hack/examples/overlays/clusterclasses/`.
  Upgrading CAREN must not roll Machines on existing clusters.
- Do not run the full e2e suite unless the change needs it.

## Security

- Report vulnerabilities through the process in `SECURITY.md`. Do not open a
  public issue for a vulnerability.
- Do not commit secrets, kubeconfigs, or credentials.
- Do not weaken the handler version rule to make a patch smaller.
