+++
title = "Handler versions"
icon = "fa-solid fa-code-branch"
+++

Upgrading CAREN must not change existing Clusters. The topology controller reconciles every Cluster after an upgrade
and applies whatever patches the handlers named in its ClusterClass return. If those patches differ from the last
release, every affected Machine is replaced. Clusters only get new behavior when their ClusterClass is changed to name a
new handler version.

## Layout

Every topology mutation handler, the mutators it composes, and every variable discovery handler live under
`pkg/handlers/vN/`. Each version has its own copy of the CRDs its variable schemas are built from in
`pkg/handlers/vN/variables/crds/`. `pkg/handlers/versions.go` registers every served version. A package under `vN` must
not import another version or any `pkg/handlers/` package outside the allowlist in `.golangci.yml`.

The highest version is open until a release contains it. Released versions are frozen. Lifecycle hooks
(`pkg/handlers/lifecycle/`) and webhooks are not versioned.

ClusterClasses name a patch handler and a discovery handler of the same version:

| Handler | ClusterClass name |
| --- | --- |
| Patches | `{provider}{cluster,worker}vNconfigpatch-gp.cluster-api-runtime-extensions-nutanix` |
| Discovery | `{provider}{cluster,worker}configvarsvN-dv.cluster-api-runtime-extensions-nutanix` |

The unversioned discovery names (`{provider}{cluster,worker}configvars-dv`) serve the `v0.50.0` schema and pair with
`v6`.

## Changing handler output

A change that alters what any handler returns for any input, including variable schemas and their defaults, goes into
the open version only. Examples: a new patch, a changed patch value, a new variable default, or a new mutator that is
enabled by default.

If the open version has been released, cut a new one first:

```shell
make handlers.new-version
```

This copies the latest `vN` to `v(N+1)`, renames its handlers, registers it, points the ClusterClass templates in
`hack/examples/overlays/clusterclasses/` at it, and runs `make examples.sync` and `make handlers.golden-update`. It
refuses to run while an unreleased version exists.

`make go-generate` copies `api/v1alpha1/crds/` into the latest version, so API changes reach only the open version.

## Golden tests

`pkg/handlers/golden` runs every registered handler over the inputs in `pkg/handlers/golden/testdata/corpus/` and
compares the patched templates, and each discovery handler's variables, byte for byte with
`pkg/handlers/golden/testdata/golden/<handler>/`. It runs as part of `make test`.

After an intended change to the open version, regenerate its golden files and review the diff:

```shell
make handlers.golden-update
```

The target only overwrites golden files of the open version. Golden files of other versions are written only when
missing.

Corpus inputs are committed copies so that changing examples or default ClusterClasses cannot change released golden
output. `hack/handlers/seed-corpus.sh` writes inputs that do not exist yet, from the generated examples and from
`hack/handlers/corpus-{minimal,features,fixtures,custom}/`. It never overwrites an existing input.

## Freeze check

`make handlers.check-frozen`, run by the `handler-freeze` workflow, fails when, compared with the last release:

- a golden file or corpus input is modified or deleted;
- a file under a released `pkg/handlers/vN/` is modified or deleted;
- a handler name registered in that release is no longer registered.

The `frozen-handler-override` label lifts the second rule only, for fixes to released handler code that leave its golden
output unchanged. Golden files, corpus inputs and handler names cannot be overridden.

## Removing a version

Clusters whose ClusterClass names a removed handler fail topology reconciliation. To remove a version, list each of its
handler names with the last release that served it in `pkg/handlers/removed-handlers.txt`. Its directory, golden files
and registration can then be deleted.

## Upgrade end-to-end test

The `Upgrade` spec installs the last release of CAREN, creates a Docker Cluster from that release's ClusterClass,
upgrades CAREN to the build under test, and fails if the KubeadmControlPlane spec, MachineDeployment specs or Cluster
variables change, or if any Machine is replaced. It runs on every pull request and nightly. To run it locally (CAPD
requires a rootful Docker daemon):

```shell
make e2e-test E2E_LABEL='provider:Docker' E2E_FOCUS='Upgrade'
```
