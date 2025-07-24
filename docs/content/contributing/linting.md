+++
title = "Linting"
icon = "fa-solid fa-magnifying-glass"
+++

This project uses [`golangci-lint`][golangci-lint] to both lint and format the CAREN sourcecode. `golangci-lint` is
installed via devbox, just as every other development tool that this project uses. The `golangci-lint` configuration
includes a custom linter, [`kube-api-linter`][kube-api-linter], integrated as a `golangci-lint` [module plugin].

## Installing `golangci-lint` with KAL

To install the customized linter binary into `hack/tools/golangci-lint-kube-api-linter`, run:

```bash
make hack/tools/golangci-lint-kube-api-linter
```

## Integrating with vscode

One the customized linter has been installed above, `vscode` can be configured to run this linter. Add the followin
configuration to `.vscode/settings.json`:

```json
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--path-mode=abs"],
  "go.formatTool": "custom",
  "go.alternateTools": {
    "customFormatter": "golangci-lint",
    "golangci-lint": "${workspaceFolder}/hack/tools/golangci-lint-kube-api-linter"
  },
  "go.formatFlags": ["fmt", "--stdin"]
```

[golangci-lint]: https://golangci-lint.run/
[kube-api-linter]: https://github.com/kubernetes-sigs/kube-api-linter/
[module plugin]: https://golangci-lint.run/plugins/module-plugins/
