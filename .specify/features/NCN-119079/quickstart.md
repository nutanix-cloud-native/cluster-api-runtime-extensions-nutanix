<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Quickstart: Support http schemes for container registries

## Prerequisites

- Go toolchain as per `go.mod`
- `devbox` and `direnv` (optional, for `devbox run`)
- Access to management cluster or ability to run unit tests

## Verify Unit Tests

```bash
go test ./pkg/handlers/generic/mutation/generic/mirrors -run Test_generateContainerd -count=1 -v
go test ./pkg/handlers/generic/mutation/generic/mirrors -run Test_needContainerdConfiguration -count=1 -v
go test ./pkg/handlers/generic/mutation/generic/mirrors -run Test_generateFiles_HTTP -count=1 -v
```

Expected: all PASS, including new http tests. Existing https tests unchanged.

## Verify Patch Output for Example

Create a Cluster with:

```yaml
- name: clusterConfig
  value:
    globalImageRegistryMirror:
      url: http://10.111.45.34/library
      credentials:
        secretRef:
          name: registry-a-credentials
    imageRegistries:
    - url: http://10.111.45.34/app-images
      credentials:
        secretRef:
          name: registry-b-credentials
```

Mutate via handler (or run `capitest` test). Check generated files:

- `/etc/containerd/certs.d/_default/hosts.toml` contains `[host."http://10.111.45.34/v2/library"]`
- `/etc/containerd/certs.d/10.111.45.34/hosts.toml` contains `server = "http://10.111.45.34"` and `[host."http://10.111.45.34"]`
- `/etc/caren/containerd/patches/registry-config.toml` present
- If secrets contain `ca.crt` with same cert, `/etc/containerd/certs.d/10.111.45.34/ca.crt` present (single file, deduplicated)
- If same host has conflicting CA, error `conflicting CA certificate specified for registry host`

## Verify No Regression for https

Mutate a Cluster with:

```yaml
- name: clusterConfig
  value:
    globalImageRegistryMirror:
      url: https://my-mirror.example.com
    imageRegistries:
    - url: https://my-registry.io
      credentials:
        secretRef:
          name: my-creds
```

Generated files should be identical to previous version (only `_default/hosts.toml` + ca.crt if CA present + drop-in). No per-host hosts.toml for https without CA.

## Lint

```bash
make fmt
make lint
make pre-commit
```

Expected: no errors in changed files.

## Docs Preview

```bash
make preview-docs
```

Check `docs/content/customization/generic/global-mirror.md` and `image-registries.md` show http examples.
