<!-- markdownlint-disable MD041 MD003 MD022 MD026 -->
<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

---
name: sync-back-pr
description: Bring internal-only work from the private fork's `internal/main` into this public repo (`nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`, CAREN), replacing the internal SDK, the internal prism-go-client fork and the internal CAPX fork with their public equivalents, leaving the private host-agent provider behind, and stripping internal-only wiring. Use when asked to "sync back to upstream", "upstream our internal changes", "open a sync-back PR", "pull internal changes into the public repo", or similar.
---

# sync-back-pr — upstream `internal/main` into this repo

This repo (`nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`, CAREN) is the public one.
A private fork, `nutanix-cloud-native/internal-cluster-api-runtime-extensions-nutanix`, carries
internal development on its `internal/main` branch. Two flows move code between them:

| | direction | branch | base | lives in |
|---|---|---|---|---|
| `sync-pr` | public → internal | `issue/sync-main` | `internal/main` | the internal fork |
| `sync-back-pr` | internal → public | `chore/sync-from-internal` | `main` | here |

In the fork, `origin` is the fork and `upstream` is this repo.

This is the higher-risk direction: it moves code out of a private repo into a public one, and it
applies a diff produced against internal's CI, registry and dependency wiring. Everything below
exists to stop something leaking, breaking public CI, or weakening this repo's security posture.

CAREN raises the stakes twice over. The release is not one image: goreleaser publishes
`runtime-extensions-components.yaml`, `metadata.yaml`, `caren-images.txt`, the Helm chart and every
`charts/cluster-api-runtime-extensions-nutanix/defaultclusterclasses/*-cluster-class.yaml` as release
assets, the chart's `values.yaml` is what every Helm user installs, and `examples/capi-quick-start/`
is what the docs point users at — a private hostname in any of them ships to users (§4). And part of
`internal/main` depends on a provider with no public release at all, so it cannot be upstreamed
however it is rewritten ("Hard blocker" below).

## Before doing anything: check for an existing sync-back PR

```bash
gh pr list --repo nutanix-cloud-native/cluster-api-runtime-extensions-nutanix --state open \
  --json number,title,headRefName,baseRefName,url
```

If a PR from `chore/sync-from-internal` is already open, update that branch in place. Never open
a duplicate.

Then check what else is in flight. A sync-back is large and will conflict with any open PR touching
`go.mod`, `api/external/`, `.github/`, `pkg/webhook/` or `charts/`:

```bash
gh pr list --repo nutanix-cloud-native/cluster-api-runtime-extensions-nutanix --state open \
  --json number,headRefName,files \
  --jq '.[] | select([.files[].path] | any(test("^(go\\.mod|api/external/|\\.github/|pkg/webhook/|charts/)"))) | "\(.number) \(.headRefName)"'
```

If one exists, **stack on it** (`--base <that branch>`) rather than racing it — merging either one
first would leave the other with a `go.mod` conflict.

## Before doing anything else: two cross-repo prerequisites

### prism-go-client

The root `go.mod` on `internal/main` carries

```
replace github.com/nutanix-cloud-native/prism-go-client => github.com/nutanix-cloud-native/internal-prism-go-client vX.Y.Z
```

and calls converged APIs that may exist **only** in the internal fork. The public repo cannot carry
that `replace` — a public module must not resolve to a private one, and a public consumer would fail
to build. So the feature has to reach public `prism-go-client` *first*. Check that the public module
actually has what the synced code calls before writing any Go:

```bash
gh api "repos/nutanix-cloud-native/prism-go-client/contents/converged?ref=main" --jq '.[].name'
grep -roh 'converged\.[A-Z][A-Za-z]*' --include='*.go' . | sort -u
```

At the time of writing public `v0.8.2` carries everything this repo uses — `PrismElementInfo`,
`StorageContainerInfo`, `APIError`, `ErrUnauthorized`/`IsUnauthorized`,
`ErrUnauthenticated`/`IsUnauthenticated`, `IsNotFound`, the OData `With*` options, `versionutils` and
the konnector client. Check anyway; the fork moves first and the gap reopens. It already has once:
the fork's `v4.WithUserAgent` client option (used by `pkg/helpers/nutanix/prismcentral.go` and the
preflight client cache, with tests asserting the `User-Agent` header) does not exist in public
prism-go-client, whose `WithUserAgent` lives in its `internal` package. The first sync-back dropped
the option and its tests; re-add them once the public module exports it.

If a symbol is missing and the feature cannot simply be left out, the public prism-go-client
sync-back has not landed yet. Do not work around it — no `replace`, no vendoring, no copying the
source in. Either wait for that repo's stack to merge and a tag to be cut, or, if the branch is
needed now, pin a pseudo-version at the head of its top-of-stack PR branch and say so plainly in the
PR body with a TODO to re-pin:

```bash
go get github.com/nutanix-cloud-native/prism-go-client@<sha-of-top-of-stack>
```

A pseudo-version pin is a **blocked** PR, not a mergeable one. Mark it as such.

### CAPX — the public release must carry every field the synced code uses

`hack/third-party/capx/go.mod` on `internal/main` carries two more replaces, to
`internal-cluster-api-provider-nutanix` and to `internal-prism-go-client`. Both go. That module exists
only so `make api.sync.capx` (`make/apis.mk`) can vendor CAPX's types into
`api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1/`, and so
`make/clusterctl.mk` can derive `CAPX_VERSION` from it with `go list -m`. `test/e2e/config/caren.yaml`,
`clusterctl.yaml` and `hack/examples/bases/nutanix/{cluster,clusterclass}/kustomization.yaml.tmpl`
build GitHub release URLs from that version, so a `replace` to the private fork silently repoints
every provider download at private releases. The fork even made the `go list` template replace-aware
(`{{ if .Replace }}{{ .Replace.Version }}{{ else }}{{ .Version }}{{ end }}`); `main`'s plain
`{{ .Version }}` comes back with it.

Pin the public module, re-vendor, and diff the vendored types against the fork's copy:

```bash
gh release list --repo nutanix-cloud-native/cluster-api-provider-nutanix --limit 5
(cd hack/third-party/capx && go mod edit -dropreplace=github.com/nutanix-cloud-native/cluster-api-provider-nutanix \
   -dropreplace=github.com/nutanix-cloud-native/prism-go-client && go get github.com/nutanix-cloud-native/cluster-api-provider-nutanix@<tag>)
make api.sync.capx
git diff internal-main -- api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/
```

Any field that diff *removes* and the synced Go references is a blocker. At the time of writing the
concrete case is `NutanixMachineSpec.VMProfile`: `pkg/handlers/nutanix/mutation/machinedetails/inject.go`
and `pkg/webhook/preflight/nutanix/vmprofile.go` use it, and public CAPX has it on `main` and in the
`v1.11.0-dev.1` prerelease but **not** in `v1.10.3`, the newest stable tag and the one this repo pins:

```bash
gh api "repos/nutanix-cloud-native/cluster-api-provider-nutanix/contents/api/v1beta1/nutanixmachine_types.go?ref=<tag>" \
  --jq .content | base64 -d | grep -c -i vmprofile
```

CAREN sits in a release chain that has to run in order: public prism-go-client merges and tags →
CAPX re-pins and releases → this repo re-pins. A sync-back is the last step's *input*. If the stable
tag lacks a field, either wait for the CAPX release or pin the prerelease and say so (a blocked PR).
Never pin the internal fork, and never hand-edit `api/external/` to paper over the gap —
`make apis.sync` runs in pre-commit and would undo it.

### The version-regression trap

Because `internal/main` resolves prism-go-client through the `replace`, the version on its `require`
line is dead text and drifts backwards — it reads `v0.8.0` while `main` is on `v0.8.2`. Applying
internal's diff therefore *downgrades* prism-go-client while appearing to leave it alone, and the
build still succeeds because the newer version is API-compatible. Nothing catches this.

It is not only prism-go-client. The fork lags `main`'s dependabot across the board: `go 1.25.0` /
`toolchain go1.26.6` against `main`'s `go 1.26.0` / `toolchain go1.27.3`, older `k8s.io/*`, ginkgo,
gomega, testify, regclient and aws-sdk pins, older public SDK pins, older action versions in
`.github/` (§3). **No version may end up lower than it is on `main`.** Check against `main`, not
against the internal branch, in every module:

```bash
git diff main -- '*.mod' | grep -E '^[-+]\s' | grep -vE '^[-+]\s*//'
```

## Hard blocker: the host-agent provider is private

CAPHA (`github.com/nutanix-cloud-native/cluster-api-provider-host-agent`) is a **private** repo with no
public release, and `internal/main` integrates it everywhere:

- Go: `hack/third-party/capha/`; vendored types under
  `api/external/github.com/nutanix-cloud-native/cluster-api-provider-host-agent/`;
  `pkg/handlers/hostagent/`; `pkg/handlers/inplaceupdate/` (in-place update, host-agent only, with
  `pkg/handlers/inplaceupdate/drain/` vendored from CAPI and
  `common/pkg/capi/clustertopology/handlers/inplaceupdate/interfaces.go`, and the
  `CanUpdateMachine` / `CanUpdateMachineSet` / `UpdateMachine` hook registration the fork adds to
  `common/pkg/server/server.go`); `pkg/webhook/preflight/hostagent/`; `api/v1alpha1/hostagent_*_types.go`;
  `internal/test/request/capha.go`; scheme and handler wiring in `cmd/main.go`.
- Generated: `api/v1alpha1/crds/caren.nutanix.com_hostagent*.yaml`;
  `charts/.../defaultclusterclasses/{host-agent,nutanix-host-agent}-cluster-class.yaml`;
  `examples/capi-quick-start/host-agent-cluster-cilium-helm-addon.yaml`; `machinedrainrules` and
  `hostagent.nutanix.com` rules in `charts/.../templates/role.yaml`.
- Chart and examples: `charts/.../templates/clusterclass_host_agent.yaml` and
  `clusterclass_nutanix_host_agent.yaml`; `hack/examples/{bases,overlays,patches,additional-resources}/hostagent/`;
  `hack/examples/overlays/clusterclasses/nutanix-host-agent/`; `oci://harbor.eng.nutanix.com/ironstack/...`
  OS images baked into those examples.
- Build and test wiring: a `host-agent` provider in `clusterctl.yaml` pointing at the private release
  URL; `CAPHA_VERSION`, `host-agent:$(CAPHA_VERSION)` and `EXP_IN_PLACE_UPDATES=true` in
  `make/clusterctl.mk`; `capha` in `make/apis.mk`; a `gh release download -R
  github.com/nutanix-cloud-native/cluster-api-provider-host-agent` block in `hack/examples/sync.sh`;
  host-agent CRD paths in `test/helpers/envtest.go`.
- Docs: `docs/content/customization/hostagent/`, `docs/content/lifecycle/in-place-update.md`,
  `.specify/features/NCN-118322/`.

Find the blast radius rather than trusting that list — it grows:

```bash
git grep -il -E 'hostagent|host-agent|host_agent|capha' internal-main   # ~120 files; zero on main
```

A Go build will not tell you any of this is a problem: `proxy.golang.org` happens to serve the capha
module zips, so `go build ./...` and even `make apis.sync` pass on a machine that can reach the proxy.
The repo is still private. `make examples.sync` runs in pre-commit, so the public `pre-commit` job
would fail on a `gh release download` its token cannot see; `make clusterctl.init` (the
`lint-test-helm` job) would fail on the private provider URL; and a dependency whose source nobody
outside Nutanix can read has no place in a public module graph.

**Rule: none of it comes back until CAPHA is public.** Strip it first, because it is also spread
through shared files:

- `pkg/handlers/nutanix/mutation/metapatch_handler.go` imports the host-agent patches into the
  *Nutanix* meta handler, for the mixed `nutanix-host-agent` ClusterClass.
- `api/v1alpha1/{addon_types,clusterconfig_types,constants,controlplane_types,nodeconfig_types}.go` and
  `api/variables/aggregate_types.go` add `HostAgent` fields — including to `NutanixWorkerNodeConfig`,
  so `caren.nutanix.com_nutanixworkernodeconfigs.yaml` carries a `hostAgent` schema.
- `provider == "host-agent"` branches in `pkg/handlers/lifecycle/cni/multus/handler.go` and
  `pkg/webhook/addons/registry/autoenabler.go` (+ `webhook_test.go`); a host-agent case in
  `pkg/handlers/lifecycle/cosi/variables_test.go`; `pkg/webhook/preflight/nutanix/{checker,specs,specs_test}.go`
  and `host_agent_project_policy*.go`.
- `charts/.../addons/cluster-autoscaler/values-template.yaml` RBAC on `hostagentmachinetemplates`; a
  capability guard in `charts/.../templates/clusterclass_nutanix.yaml`; host-agent paragraphs in
  `docs/content/addons/{cluster-autoscaler,registry}.md`.
- `charts/.../addons/csi/nutanix/values-template.yaml` plus the new
  `pkg/handlers/lifecycle/csi/nutanix/template.go` compute `BareMetalMode` as
  `GetProvider(cluster) == "host-agent" || <ExperimentalNutanixCSIBaremetalMode annotation>`. The
  annotation half is portable; the provider half is not. Split it.

Once the Go is gone, regenerate rather than edit the outputs (§5): `make go-generate` drops the
`machinedrainrules` and `hostagent.nutanix.com` rules from `role.yaml` by itself (they come from
kubebuilder RBAC markers in the deleted packages), and `make examples.sync` stops producing the
host-agent ClusterClasses — but `hack/examples/sync.sh` only writes, it never prunes, so delete the
three stale generated files by hand first. State the exclusion, and the reason, in the PR body.

## Working safely

Use a scratch worktree so the diff can be shaped without disturbing a checkout:

```bash
git worktree add /tmp/sync-back main
cd /tmp/sync-back
git fetch git@github.com:nutanix-cloud-native/internal-cluster-api-runtime-extensions-nutanix.git internal/main:internal-main
git checkout -b chore/sync-from-internal internal-main
```

Strip host-agent, rewrite dependencies and strip internal-only files (below), then squash onto `main`.

## 1. Dependencies: public SDK only, on `main`'s versions or newer

Rewrite every internal SDK import to its public equivalent:

```
github.com/nutanix-core/ntnx-api-golang-sdk-internal/<mod>-go-client/v17
  ->  github.com/nutanix/ntnx-api-golang-clients/<mod>-go-client/v4
```

At the time of writing that covers `clustermgmt`, `multidomain`, `networking`, `prism` and `vmm` as
direct requirements, plus `datapolicies`, `iam`, `monitoring` and `volumes` as indirects. The model
sub-paths are identical between the two SDKs — `models/clustermgmt/v4/config`,
`models/clustermgmt/v4/ahv/config`, `models/clustermgmt/v4/error`, `models/multidomain/v4/config`,
`models/networking/v4/config`, `models/prism/v4/config`, `models/vmm/v4/content`,
`models/vmm/v4/ahv/config`, `models/vmm/v4/error`, `models/common/v1/config` — so the rewrite is purely
the module prefix and major version; import aliases and every `clustermgmtv4.` / `vmmv4.` reference
stay as they are. Note that `models/common/v1/config` is imported from **three** modules
(`multidomain`, `networking`, `prism`); each ships its own copy, so keep each import under the module
it came from.

The imports live in `pkg/webhook/preflight/nutanix/*.go` and `test/e2e/framework/nutanix/*.go`. The
latter is behind `//go:build e2e`, so a plain build never compiles it — see §7.

**There are nine modules** (ten on `internal/main`, counting `hack/third-party/capha`, which goes):
the root `go.mod`, `api/go.mod`, `common/go.mod`, `docs/go.mod`, `hack/tools/go.mod` and one per
`hack/third-party/{capa,caaph,capx,metallb}/go.mod`. Only two carry internal wiring — the root (the
SDK requires, the prism-go-client `replace`, and a dangling
`replace .../cluster-api-runtime-extensions-nutanix/internal => ./internal` for a directory that has no
`go.mod`) and `hack/third-party/capx` (two replaces plus a `google.golang.org/genproto` pin) — but all
of them regress versions. The least error-prone order:

1. Rewrite the imports in the Go files.
2. Restore `main`'s `go.mod` and `go.sum` in every module (`git checkout main -- go.mod go.sum
   api/go.mod api/go.sum ...`), so no pin can go backwards.
3. `make mod-tidy` (runs `go mod tidy` in every module, third-party ones included). It promotes
   `multidomain` to a direct requirement and adds anything else the synced code imports.
4. `go get` only what the synced code needs and `main` lacks, then verify.

Pin any SDK module you had to add or bump to its newest tag, reading tags from the source of truth
rather than the module proxy, which can lag:

```bash
git ls-remote --tags https://github.com/nutanix/ntnx-api-golang-clients.git \
  | awk '{print $2}' | sed 's|refs/tags/||' | grep '^<mod>-go-client/' | sort -V | tail -5
```

**Gotcha that has bitten before:** if a `go.mod` still holds `v17.x` versions under a `/v4` module
path, `go get` aborts with *"invalid: should be v4, not v17"* and applies **none** of the requested
pins. It reads like a warning; it is a total no-op. Step 2 avoids it; otherwise fix the versions by
hand, then `go get`, then verify every line (`grep -h ntnx-api go.mod api/go.mod common/go.mod`).

Prefer stable tags. A prerelease is acceptable only where a needed package genuinely does not exist
in the stable tag — internal features reach public betas first. `multidomain` is the usual offender
(`main` is already on a `-beta` of it for the projects API); `iam` is the other. `go mod tidy` names
the exact missing package. If a beta is required, say so in the PR body and name the package.

## 2. No internal wiring may remain

```bash
grep -rn "ntnx-api-golang-sdk-internal\|nutanix-core/ntnx-api\|internal-prism-go-client" \
  --include="*.go" --include="*.mod" --include="*.sum" .
```

Must come back empty in **every** module — `go.sum` included.

Internal hostnames, private sibling repos and product-artifact paths leak just as badly as Go
imports, and in this repo they hide in *generated* YAML (chart values, chart README, examples, e2e
config), which lets them survive a careful review. First pass without `go.sum`, so the output is
readable; final pass with it:

```bash
grep -rn -E "ntnx-api-golang-sdk-internal|nutanix-core/ntnx-api|internal-prism-go-client|internal-cluster-api-provider-nutanix|internal-cluster-api-runtime-extensions|harbor\.eng\.nutanix\.com|ncn-prerelease|dkp-container-images|ironstack|GOPRIVATE|cluster-api-provider-host-agent" \
  --exclude-dir=.git --exclude='*.sum' .
grep -rn -E "<same pattern>" --exclude-dir=.git .
```

Two hits are expected on `main` and are not leaks: `make/go.mk` declares `export GOPRIVATE ?=` (an
empty default), and `make/addons.mk` names `github.com/nutanix-core/k8s-agent` and
`nutanix-core/flow-k8s-cni` in comments, because those are the real upstream repos of the Konnector
agent and Flow CNI. Anything else is a finding.

Also drop internal-only tooling that has no meaning here: the fork's `sync-pr` skill at
`.agents/skills/sync-pr/` and the symlink that points at it (`find . -type l -lname '*sync-pr*'`).

**Do not delete this skill.** `sync-back-pr` lives in this repo at
`.agents/skills/sync-back-pr/SKILL.md`, with a committed symlink (mode `120000`) next to it, mirroring
the fork's layout. Depending on when the fork last ran `sync-pr`, `internal/main` may not contain it,
in which case the sync-back diff will show it as a deletion. Keep it — same hazard class as the
deletions catalogued in §3.

## 3. `.github/` — the security-critical part

**Read every hunk and decide on it.** Do not reach for `git checkout main -- .github/`. A blanket
restore looks safe because it cannot leak anything, but it is a decision not to think, and it
silently reverts changes that genuinely belong here — most reliably the CI wiring for tests arriving
in the same sync, which then run unconfigured or not at all. Wholesale-keeping is obviously wrong;
wholesale-reverting is wrong in a way that passes CI.

Work through the diff hunk by hunk:

```bash
git diff main -- .github/
```

Every hunk is one of four things, and you have to say which:

1. **Internal-only infrastructure** → drop. Credentials, private registries, private module access,
   internal branch names. Catalogued below.
2. **A deletion or disabling of something only the public repo needs** → drop the change. The fork
   has no external contributors, no public issue tracker and no public docs site, so its CI and repo
   metadata legitimately lack things this repo depends on. Catalogued below.
3. **A runner or trigger change** → drop, and treat it as a finding rather than a hunk. Catalogued
   below.
4. **Genuinely portable CI** → keep. Wiring for tests this sync adds, a real fix to a job both repos
   run. Keep it, and justify it in the PR body.

The catalogues below are the lens, not the whole answer — they list what has come up before, and a
hunk that fits none of them still needs a judgement. When a hunk mixes categories, split it: the
fork's `Run e2e tests` step in `e2e.yml` carries six new project-scoped `env:` entries (keep), an
`ARTIFACTS=` argument that only feeds an S3 upload (drop) and a `GITHUB_TOKEN` swapped for an app
token (drop), in the same dozen lines.

### Internal additions that must NOT land here

- `env: GOPRIVATE: ${{ vars.GOPRIVATE }}` / `GONOSUMDB` blocks — meaningless once dependencies are
  public. Added to `blackduck.yaml`, `checks.yml`, `codeql-analysis.yml`, `devbox-update.yml`,
  `e2e.yml`, `govulncheck.yml` and `release-tag.yml`.
- The paired `actions/create-github-app-token@v3` steps (`GHA_CHECKOUT_APP_ID` /
  `GHA_CHECKOUT_APP_PRIVATE_KEY`, owners `nutanix-cloud-native` and `nutanix-core`), the
  `git config --global url."https://x-access-token:...".insteadOf` rewrites, and the `GH_TOKEN` /
  `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` swapped for that token in the `pre-commit` step, the
  `lint-test-helm` `clusterctl.init` step and the e2e run step. These mint credentials for private
  orgs. This repo runs workflows for **fork PRs**; token-minting steps here are a credential-exposure
  risk. Restore `secrets.GITHUB_TOKEN`.
- `DeterminateSystems/nix-installer-action` plus `skip-nix-installation: true` and
  `enable-cache: false` on `jetify-com/devbox-install-action`, wherever the fork adds them to a job
  this repo runs on `ubuntu-24.04`. ARC-runner plumbing; it goes with the runner move. (`e2e.yml` on
  `main` already has the nix step, guarded by `if: inputs.runs-on != 'ubuntu-24.04'`; not this hazard.)
- `internal/main` and `internal/release/v*` branch triggers in `blackduck.yaml`, `checks.yml`,
  `govulncheck.yml` and `release-please.yml`.
- `if: github.repository == 'nutanix-cloud-native/internal-cluster-api-runtime-extensions-nutanix'`
  on the Black Duck job. It never matches here, so the job silently never runs. A permanently-skipped
  required check is worse than a failing one — it looks green. Restore this repo's name.
- Harbor: the `docker/login-action` to `harbor.eng.nutanix.com` with `HARBOR_USERNAME` /
  `HARBOR_PASSWORD` in `release-tag.yml`, and the `lint-test-helm` `kind load docker-image` of the
  `harbor.eng.nutanix.com/ncn-prerelease/internal-...-helm-chart-bundle-initializer` image in place of
  the `ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer` image
  that `.goreleaser.yml` builds here (§4).
- The "Upload test artifacts to S3" step in `e2e.yml`, with `S3_ARTIFACTS_*` secrets and vars that do
  not exist here, the `ARTIFACTS: ${{ github.workspace }}/_artifacts` env and `ARTIFACTS=` argument
  that exist only to feed it, and `awscli2` in `devbox.json`. One hunk; all internal.
- The extra `self-hosted-nutanix-*` labels in `.github/actionlint.yaml`, the `pull-requests: read`
  top-level permission in `checks.yml` and the `ref: ${{ github.event.pull_request.head.sha }}`
  checkout in the `pre-commit` job. Fork-specific; nothing here needs them.
- **Versions that go down.** `actions/cache@v6 → v5`, `actions/setup-java@v6 → v5` (with
  `temurin → adopt`), `actions/setup-go@v7 → v6`, `nix-installer-action@v23 → v22`, and the `lint-go`
  reviewdog command, where `main` exports `GOTOOLCHAIN` from `go.mod` before running the custom linter
  and the fork's single `bash -c` pipeline predates that fix. These are the fork lagging `main`'s
  dependabot. An action version going down is never a sync hunk.

### Internal removals that must NOT propagate here

A sync-back applies internal's *diff*, so a file deleted in the fork gets deleted here too. This
repo's public-facing surface lives entirely in things the fork has no use for, which makes this the
most dangerous section on the page. Guard against each explicitly:

- **Repo metadata.** `.github/CODEOWNERS` and `.github/ISSUE_TEMPLATE/{bug_report.md,config.yml,feature_request.md}`
  do not exist on `internal/main`. Outside `.github/`, neither do `AGENTS.md`, `CODE_OF_CONDUCT.md`,
  `CONTRIBUTING.md`, `OWNERS`, `SECURITY.md` or `SUPPORT.md`; `README.md` is cut down to a stub and
  `docs/content/contributing/_index.md` loses its links to them. The sync-back presents all of it as
  deletions. Every one stays, at `main`'s content.
- **The docs site.** `github-pages.yml` has its `push` and `pull_request` triggers removed on
  `internal/main` (the fork's site is disabled; `workflow_dispatch` only) and `release-tag.yml` drops
  the final "Build GitHub pages" `benc-uk/workflow-dispatch` step. The public site at
  `nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix` depends on both. Restore both.
- **CodeQL.** The fork keeps `codeql-analysis.yml` but moves it (below). Any hunk that disables,
  guards or `continue-on-error`s it is category 2.
- This repo's `branches: [main, release/v*]` triggers.

### Runner changes — never carry these back

The fork moves every job in every workflow except `e2e.yml` off `ubuntu-24.04` onto
`self-hosted-nutanix-small` / `-medium` / `-docker-small` / `-docker-medium` — `backport.yml`,
`blackduck.yaml`, every job in `checks.yml`, `codeql-analysis.yml`, the conventional-label/-pr-title
and dependabot workflows, `devbox-update.yml`, all three jobs in `github-pages.yml`, `govulncheck.yml`,
`release-please.yml`, `release-tag.yml`, `stacked-prs.yml` — and collapses the e2e `runs-on` expression
that `main` keeps as `${{ matrix.config.provider == 'Nutanix' && 'self-hosted-nutanix-docker-medium' || 'ubuntu-24.04' }}`
to the self-hosted label unconditionally, which would put the Docker-provider e2e runs on Nutanix
hardware. That is fine for a private repo where every contributor is trusted. Carrying it here would
let a fork PR execute attacker code on Nutanix-controlled runners.

**Rule:** jobs in this repo stay on the runner `main` puts them on, and CodeQL stays GitHub-hosted:
untrusted fork code must compile in an ephemeral sandbox, not on self-hosted infrastructure. If a
sync-back diff moves a job onto a self-hosted runner, or introduces `pull_request_target` where this
repo used `pull_request`, stop — that is a security regression, not a sync.

### Category 4 in practice: wiring for tests you just added

This is the category a reviewer is least likely to miss and an automated sweep is most likely to
throw away. If the sync-back brings **new tests that read CI-supplied credentials**, the `env:` lines
feeding them are part of the change, not internal-specific wiring.

Getting it wrong is worse than it sounds, because the project-scoped e2e specs
(`test/e2e/nutanix_projects_test.go`, `nutanix_projects_helpers.go`) `Skip()` when their variables are
unset rather than failing. Omit the wiring and CI stays green while the new scenarios never run once —
the same "permanently-skipped check looks green" hazard as a `github.repository ==` guard, arriving
from the opposite direction.

What has come up so far, all verified against the fork:

- The new `e2e-nutanix-projects` job in `checks.yml`: `uses: ./.github/workflows/e2e.yml` with
  `focus: Nutanix project`, `provider: Nutanix`, a `base-os`, `secrets: inherit`, and
  `runs-on: self-hosted-nutanix-docker-medium` — which is what `main` already uses for every Nutanix
  e2e job, so it is not a runner move. Set its `kubernetes-version` / `base-os` to the newest Nutanix
  entry of `main`'s `e2e-quick-start` matrix, not to the fork's literal (the fork's matrix is older).
- The new `env:` entries on the `Run e2e tests` step in `e2e.yml`:

  ```yaml
  NUTANIX_API_KEY: ${{ secrets.NUTANIX_API_KEY }}
  NUTANIX_PROJECT_SCOPE_USER: ${{ secrets.NUTANIX_PROJECT_SCOPE_USER }}
  NUTANIX_PROJECT_SCOPE_PASSWORD: ${{ secrets.NUTANIX_PROJECT_SCOPE_PASSWORD }}
  NUTANIX_PROJECT_NAME: ${{ vars.NUTANIX_PROJECT_NAME }}
  NUTANIX_INACCESSIBLE_PROJECT_NAME: ${{ vars.NUTANIX_INACCESSIBLE_PROJECT_NAME }}
  NUTANIX_PROJECT_STORAGE_CONTAINER_NAME: ${{ vars.NUTANIX_PROJECT_STORAGE_CONTAINER_NAME }}
  ```

  `checks.yml` calls `e2e.yml` with `secrets: inherit`, so no `secrets:` declaration is needed on the
  reusable workflow — a bare reference is enough.
- The junit publish step's condition changed from `success() || failure()` to `"!cancelled()"`, and
  `github_token: ${{ secrets.GITHUB_TOKEN }}` on the actionlint reviewdog step. Small real fixes to
  jobs both repos run.

Carry those in the diff, then check the names resolve. **Repo secrets and variables live in repo
settings, not in the diff**, so a sync-back cannot create them and the reviewer cannot see they are
missing:

```bash
for r in cluster-api-runtime-extensions-nutanix internal-cluster-api-runtime-extensions-nutanix; do
  gh variable list --repo nutanix-cloud-native/$r; gh secret list --repo nutanix-cloud-native/$r
done
```

At the time of writing this repo has vars `E2E_DEFAULT_KUBERNETES_VERSION`, `NUTANIX_PORT`,
`NUTANIX_PRISM_ELEMENT_CLUSTER_NAME`, `NUTANIX_STORAGE_CONTAINER_NAME`, `NUTANIX_SUBNET_NAME` and
secrets `BLACKDUCK_*`, `DOCKER_HUB_*`, `LOCAL_IMAGE_REGISTRY*`, `NUTANIX_ENDPOINT`,
`NUTANIX_FLOW_DOCKER_HUB_*`, `NUTANIX_PASSWORD`, `NUTANIX_USER`. The fork additionally has vars
`NUTANIX_PROJECT_NAME`, `NUTANIX_INACCESSIBLE_PROJECT_NAME`, `NUTANIX_PROJECT_STORAGE_CONTAINER_NAME`
and secrets `NUTANIX_PROJECT_SCOPE_USER`, `NUTANIX_PROJECT_SCOPE_PASSWORD`. Nobody has
`NUTANIX_API_KEY`. Anything referenced but absent must be created by someone with admin on this repo
before the tests mean anything — call it out in the PR body as a required follow-up, with the names,
since it is work the PR itself cannot do.

### Before opening the PR

Read the surviving diff once more, as a whole:

```bash
git diff main -- .github/
```

There is no expected size. What there is, is an expectation that you can name the reason for every
remaining hunk, and that each one is category 4. List them in the PR body with those reasons. A
`.github/` diff nobody can explain hunk-by-hunk is the failure this section exists to prevent —
whether it is fifty lines long or zero.

## 4. Release artifacts, charts, examples, e2e config — the second leak surface

This is where CAREN differs most from the other repos, and where a leak is worst, because so much of
the tree is generated, committed and then published or installed.

### Release pipeline

`.goreleaser.yml` on `internal/main` renames the `kos` repository and the helm-chart-bundle-initializer
image to `ghcr.io/nutanix-cloud-native/internal-cluster-api-runtime-extensions-*`, duplicates both to
`harbor.eng.nutanix.com/ncn-prerelease/internal-...`, and adds two release assets,
`runtime-extensions-components-public.yaml` and `caren-images-public.txt`, built by new
`release.public-images-list` / `release.public-manifest` targets in `make/goreleaser.mk` and
`hack/release/public-images/kustomization.yaml` (which rewrites Harbor images to
`ghcr.io/nutanix-cloud-native/dkp-container-images/nkp-...`, an NKP product artifact), with matching
`.gitignore` entries. **None of it comes back.** This repo publishes
`ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix` and
`ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer`, nothing
else. `main` is also on goreleaser's `dockers_v2` block (one multi-platform entry, `sbom: false`, an
explicit `main: .` on the ko build) while the fork still has the older `dockers` + `docker_manifests`
form, so the diff would revert that modernisation too. Restore `main`'s file unless you can name a
non-registry change in the diff.

### Chart, dev loop and e2e config

- `charts/cluster-api-runtime-extensions-nutanix/values.yaml`: `image.repository` and
  `helmRepository.images.bundleInitializer.repository` repointed at Harbor; `Chart.yaml` `home` /
  `sources` and the helm-docs-generated `README.md` repointed at the internal repo. Restore `main`'s
  values and regenerate the README with `make chart-docs` (and `values.schema.json` with
  `make schema-chart`) rather than editing them.
- `make/dev.mk` `dev.run-on-kind` loads the Harbor bundle image into kind. Restore GHCR.
- `test/e2e/config/caren.yaml`: the `images:` entry names the Harbor bundle image and the `nutanix`
  provider `value:` names `internal-cluster-api-provider-nutanix`. Both must be public — the provider
  stays `https://github.com/nutanix-cloud-native/cluster-api-provider-nutanix/releases/download/${CAPX_VERSION}/infrastructure-components.yaml`.
- `test/e2e/e2e_suite_test.go` gains `preFetchPrivateProviderComponents`, which shells out to
  `gh release download --repo nutanix-cloud-native/internal-cluster-api-provider-nutanix` for any
  provider URL starting `https://github.com/nutanix-cloud-native/internal-`. Drop it and its call.
- `hack/examples/sync.sh` gains `gh release download -R github.com/nutanix-cloud-native/internal-cluster-api-provider-nutanix`
  into `hack/examples/bases/nutanix/downloads/`, and
  `hack/examples/bases/nutanix/{cluster,clusterclass}/kustomization.yaml.tmpl` switch from the public
  release URL resources to `../downloads/*.yaml`. Restore the URL resources; drop the download block
  and the `downloads` dirs. `clusterctl.yaml` likewise gains a `nutanix` override pointing at the fork
  (`main` has none; clusterctl's built-in entry is already the public repo). Drop it.

Keep the genuinely portable parts of those same files: the `*-with-project` and
`*-with-project-scoped-user` cluster templates and their `sourcePath` entries in `caren.yaml`; the
`NUTANIX_API_KEY`, `NUTANIX_PROJECT_SCOPE_*`, `NUTANIX_PROJECT_NAME`, `NUTANIX_INACCESSIBLE_PROJECT_NAME`
and `NUTANIX_PROJECT_STORAGE_CONTAINER_NAME` variables; and the nutanix-auth overlay selection
(`NUTANIX_USE_AUTH_OVERLAYS`, `NUTANIX_CREDENTIALS_TYPE`) in `sync.sh` and `make/examples.mk` with its
`hack/examples/{overlays/clusters,patches}/nutanix-auth/` trees. Then run the §2 grep over the
regenerated `examples/capi-quick-start/*.yaml` and `defaultclusterclasses/*.yaml` rather than assuming
— they are generated files that a future bump could re-point.

### Release bookkeeping is per-repo

Each repo runs release-please on its own line, so these stay exactly as `main` has them:
`.release-please-manifest.json` (the fork is on 0.65.x; this repo on 0.50.x), `CHANGELOG.md`,
`docs/hugo.toml` `version`, the `releaseSeries` entries appended to `metadata.yaml` and
`test/e2e/data/shared/caren/metadata.yaml` (release-please adds them on the public release), and the
"vNext" provider version in `test/e2e/config/caren.yaml` (`v0.51.99` on `main`; the fork moved it to
its own line). `make/kind.mk` `KIND_CLUSTER_NAME`, the `release-please` branch filter and
`--latest-tag-name` flag in `make/dev.mk`, and `docs/package-lock.json` drift are fork-specific too.

### Addon pins are a release-chain question

The fork tracks NKP builds, so its addon pins name charts and images this repo's users cannot or
should not get. At the time of writing, in `make/addons.mk` and
`hack/addons/kustomize/*/kustomization.yaml.tmpl`:

- Nutanix CCM: `nkp-cloud-provider-nutanix` 0.9.x from
  `oci://ghcr.io/nutanix-cloud-native/dkp-container-images/chart`, an NKP product chart. `main` pins
  the public `nutanix-cloud-provider` chart (from `oci://ghcr.io/nutanix-cloud-native/chart` since
  the chart moved off the classic helm index). Keep `main`'s `NUTANIX_CCM_CHART_VERSION` and chart
  source. A CCM bump belongs to the follow-up that re-pins after the public CCM release, not inside
  the sync.
- Konnector Agent: 1.5.x from `https://mesosphere.github.io/charts/stable/` plus an image override
  `quay.io/karbon/k8s-agent:1.5.1-rc.1` in `charts/.../addons/konnector-agent/values-template.yaml`.
  The public index at `https://nutanix.github.io/helm-releases` tops out at 1.5.0 and `main`
  deliberately stays on 1.4.2 (the comment says why: 1.5.0 hung `BeforeClusterDelete` in e2e). Keep
  `main`'s `KONNECTOR_AGENT_VERSION` and repo, drop the RC image override, keep the `projectId` /
  `projectName` template additions — value additions the synced handler fills. Dropping the override
  has a dependency: the fork's `hack/tools/fetch-images/main.go` grew a `konnector-agent` case that
  reads `agent.image.repository` from that values template and fails `check-list-images` with
  "missing agent.image.repository" when it is gone. Drop the case and its `getKonnectorAgentImage`
  helper with the override; the public chart's default image is listed like every other addon's.
  Also update the version line in `docs/content/addons/konnector-agent.md` to match the pin.
- cluster-autoscaler: the `image.tag` override moves from `v1.34.3` to `v1.36.1` with a comment
  explaining the kubernetes/autoscaler#9439 fix. The image is public and the comment is sound, so it
  is portable on its face — but it is an addon behaviour change. Carry it with its comment and name it
  in the PR body.
- `charts/.../addons/csi/nutanix/values-template.yaml` adds `pulse.enabled: true` and
  `ntnxInitConfigMap.isNKP: true`. NKP telemetry markers, not a leak; whether the public chart should
  default to them is a reviewer decision. Flag it, do not decide it silently.

Whatever the pins end up as, regenerate `charts/.../templates/helm-config.yaml` and
`hack/addons/helm-chart-bundler/repos.yaml` with `make template-helm-repository`; never edit them.
The sources are the kustomization templates and `make/addons.mk`.

## 5. Regenerate rather than hand-edit

Most of this repo's YAML is generated, and a sync-back that edits outputs directly will pass review
and then fail the first pre-commit run that regenerates:

```bash
make go-generate               # api/v1alpha1/crds, deepcopy, charts/.../templates/{role,webhooks}.yaml
make apis.sync                 # api/external/ from hack/third-party/*/go.mod
make addons.sync               # charts/.../templates/*/manifests, then template-helm-repository
make template-helm-repository  # charts/.../templates/helm-config.yaml, hack/addons/helm-chart-bundler/repos.yaml
make examples.sync             # charts/.../defaultclusterclasses/*.yaml, examples/capi-quick-start/*.yaml
make chart-docs schema-chart   # charts/.../README.md, charts/.../values.schema.json
make list-images               # the check-list-images hook: pulls every chart in helm-config.yaml
git diff --exit-code           # must be empty
```

`make examples.sync` needs `CAPX_VERSION`, `CAPA_VERSION` and `CAAPH_VERSION`, which `make/clusterctl.mk`
derives from the third-party `go.mod` files — so it is also the first real check that the CAPX
prerequisite was done, since it fetches the public CAPX release templates for that version.
`make list-images` pulls every chart named in `helm-config.yaml`; an OCI or private chart source left
in the addon pins breaks it. `devbox run -- make pre-commit` runs all of the above as hooks (plus
`go-mod-tidy` per module, license headers, actionlint, shellcheck, markdownlint) and fails if anything
changes; run it until it is clean. Unit tests are **not** a pre-commit hook (§7).

Expect formatting-only churn in the Go diff — `))` against `),\n)` closers in files such as
`pkg/webhook/addons/registry/autoenabler.go` and `test/e2e/e2e_suite_test.go`. That is tooling drift:
`main`'s `devbox.lock` is months newer than the fork's, so `golangci-lint fmt` disagrees with itself
across the two. Keep `main`'s `devbox.json`, `devbox.lock` and `.golangci.yml`, run `make fmt`, and do
not hand-pick formatting hunks.

## 6. Commit shape

Squash into a single commit on top of `main`. The internal history is dominated by release-please
commits, `sync internal/main with upstream main` merges and internal Jira IDs, and a merge would
import commits whose content this skill then reverts — incoherent public history.

Squashing costs per-commit authorship, so preserve it explicitly:

```bash
git log --format='%an <%ae>' main..internal-main | sort -u \
  | grep -viE "noreply@github|dependabot|actions@github|copilot|release-please"
```

Add each as a `Co-authored-by:` trailer, one per email — the same person often appears under two
names. This is a public repo; do not erase contributors.

Per global instructions, no Claude/Anthropic attribution in commit messages or PR descriptions.

## 7. Verify before opening the PR

```bash
go build ./... && go vet ./...
go vet -tags e2e ./test/e2e/...
go test -tags e2e -run '^$' ./test/e2e/...   # compile/smoke check, provisions nothing
devbox run -- make test                      # root, api/ and common/; uses setup-envtest
devbox run -- make lint                      # builds hack/tools/golangci-lint-kube-api-linter with the root toolchain
devbox run -- make pre-commit                # §5, until clean
```

The `-tags e2e` runs are not optional. `test/e2e` is in the root module but behind a build tag, so a
plain `./...` build never compiles it — and it is where half the SDK imports live. The single most
likely way for this sync to ship broken code is an e2e package that nobody built.

Expect lint findings the fork never saw. The `lint-go` job reports through reviewdog with
`--filter-mode=added`, so on `internal/main` the fork's own lint debt (goconst on repeated `"nutanix"`,
`"name"` and `"value"` literals in `test/e2e`, a gocyclo breach in
`pkg/webhook/preflight/nutanix/project_consistency.go`, type-before-method ordering in
`aosversion.go`) hides behind lines nobody touched. In the sync-back every one of those lines is
"added", so they all fail the PR. Fix them — reuse the constants `main` already has (the JSONPath
field constants in `pkg/webhook/preflight/nutanix/constants.go`, `nutanixProvider` in
`test/e2e/quick_start_helpers.go`) rather than adding `//nolint` across the board.

The Nutanix quick-start and projects e2e suites provision real VMs against a live Prism Central and
need `NUTANIX_ENDPOINT`, `NUTANIX_USER` / `NUTANIX_PASSWORD`, `NUTANIX_PORT`,
`NUTANIX_PRISM_ELEMENT_CLUSTER_NAME`, `NUTANIX_SUBNET_NAME`, `NUTANIX_STORAGE_CONTAINER_NAME`, the
Docker Hub credentials and — for the projects suite — the five project variables from §3 plus a PC
with projects configured. They cannot be run locally. If you cannot reach such an environment, say so
plainly in the PR body rather than implying the sync was verified end to end, and list exactly what
was run.

## 8. PR body

Title: `feat: sync internal fork changes using public API SDKs`

Keep the description concise: what changed, what was deliberately excluded, what a reviewer needs to
look at, how it was tested. No emoji headers, no narration of how the diff was produced. Include:

1. **What is being upstreamed** — the new public API surface, in a paragraph.
2. **Dependency versions** — the public prism-go-client version and the public SDK pins, with an
   explicit statement that neither went backwards relative to `main`; the public CAPX tag and whether
   it is a prerelease; any pseudo-version pin with the reason and the package that forces it.
3. **What was deliberately excluded and why** — host-agent and in-place-update (CAPHA is private; the
   handlers, CRDs, examples, docs and `make` / `clusterctl.yaml` wiring are all left behind), internal
   CI and Harbor wiring, `hack/release/public-images`, the vendored internal CAPX templates, the
   `sync-pr` skill, and the addon pins kept at `main`'s values.
4. **Every surviving `.github/` hunk**, with its reason.
5. **Missing repo secrets and variables**, by name, as an admin follow-up.
6. **Anything a reviewer would otherwise miss** — CRD or API field changes, webhook behaviour changes
   (new preflight checks, new defaulters), addon default changes (cluster-autoscaler tag, the CSI
   telemetry values), exported-constant renames, inverted test assertions.
7. **Honest test status**, including what was not run.

## Checklist

- [ ] No duplicate sync-back PR; stacked on any conflicting in-flight PR
- [ ] Public prism-go-client carries every `converged.*` symbol the synced code calls
- [ ] Public CAPX tag carries every vendored field the synced code uses (`vmProfile` et al.); a
      prerelease pin, if any, justified in the PR body
- [ ] No `replace` to `internal-prism-go-client` or `internal-cluster-api-provider-nutanix` in any module
- [ ] All nine `go.mod` files cleaned; no version lower than `main`'s, prism-go-client and the public
      SDK pins included; `hack/third-party/capha` gone
- [ ] Zero `ntnx-api-golang-sdk-internal` / `nutanix-core/ntnx-api` references, `go.sum` included
- [ ] Zero `harbor.eng.nutanix.com`, `ncn-prerelease`, `dkp-container-images`, `ironstack`,
      `cluster-api-provider-host-agent` or private-sibling-repo references anywhere, generated YAML
      included; only the two known `main` hits remain
- [ ] Nothing host-agent or in-place-update carried: `git grep -il -E 'hostagent|host-agent|host_agent|capha'`
      is empty
- [ ] `hack/examples` and `test/e2e/config/caren.yaml` resolve CAPX from the public release URL; no
      `downloads/` dirs, no `preFetchPrivateProviderComponents`, no provider overrides in `clusterctl.yaml`
- [ ] `.goreleaser.yml`, chart `values.yaml`, `Chart.yaml`, chart `README.md`, `make/dev.mk` and the e2e
      `images:` entry name public registries and this repo only; no `hack/release/public-images`
- [ ] Release bookkeeping untouched: `.release-please-manifest.json`, `CHANGELOG.md`, `docs/hugo.toml`,
      both `metadata.yaml` files, the vNext version, `make/kind.mk`, `make/dev.mk` release-please lines
- [ ] Addon pins at `main`'s values and sources; RC image override dropped; `helm-config.yaml` and
      `repos.yaml` regenerated; CSI telemetry values flagged for the reviewer
- [ ] `CODEOWNERS`, `ISSUE_TEMPLATE/`, `AGENTS.md`, `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `OWNERS`,
      `SECURITY.md`, `SUPPORT.md` and the full `README.md` still present
- [ ] `github-pages.yml` `push` / `pull_request` triggers and the `release-tag.yml` pages dispatch still
      present; Black Duck guard names this repo
- [ ] No job moved to a self-hosted runner, e2e `runs-on` expression intact; no app-token steps, no
      `insteadOf` rewrites, no `GOPRIVATE`, no Harbor login, no S3 upload, no action version lowered
- [ ] `e2e-nutanix-projects` job and the six new e2e `env:` entries kept; missing secrets/vars named in
      the PR body
- [ ] Generated trees regenerated, not hand-edited; stale host-agent outputs deleted;
      `git diff --exit-code` clean after `devbox run -- make pre-commit`
- [ ] `sync-pr` skill and its symlink not carried over; this skill not deleted
- [ ] Single squashed commit with `Co-authored-by:` trailers for every human author
- [ ] Build, vet, `make test`, `make lint` and `make pre-commit` pass, **including the `-tags e2e`
      compile**; unrun e2e disclosed in the PR body
