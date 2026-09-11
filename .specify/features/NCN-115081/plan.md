<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Implementation Plan: Keyless signing and provenance attestation for release artifacts

**Branch**: `NCN-115081-keyless-signing-attestation`
**Date**: 2026-06-10
**Spec**: [./spec.md](./spec.md)

## Summary

Add keyless (Sigstore/Fulcio/Rekor, GitHub Actions OIDC) cosign signatures and
SLSA build-provenance attestations to the CAREN release pipeline for both
container images and the Helm chart, and additionally publish the Helm chart to
`ghcr.io` as an OCI artifact so it can be signed and attested by digest exactly
like the images.

All work is confined to `.github/workflows/release-tag.yml` and `devbox.json`.
The goreleaser build itself is unchanged — signing and attestation run as
workflow steps *after* `make release` has pushed the images, so the `ko`-built
CAREN image and the buildx-built bundle-initializer image are treated
identically.

Image and chart digests are resolved with `crane digest` (already in devbox)
against the pushed references, rather than parsing `dist/artifacts.json`. This
is registry-truth, mechanism-agnostic (ko vs buildx vs helm), and gives the
multi-arch index digest directly.

## Technical Context

**Language/Version**: GitHub Actions workflow YAML; bash; devbox-managed CLIs.
**Primary Dependencies**: `cosign` (new, via devbox), `oras` (new, via
devbox), `crane` (existing, devbox), `helm` (existing, via
`hack/flakes#helm-with-plugins`), `gh` (existing),
`actions/attest-build-provenance@v3` (new action).
**Storage**: `ghcr.io` (images, OCI chart, cosign signatures, attestation
referrers); GitHub attestations API; Sigstore Rekor transparency log.
**Testing**: `actionlint` on the workflow; `devbox run -- make
release-snapshot` to confirm goreleaser still builds; post-release `cosign
verify` / `gh attestation verify` as acceptance checks. A real end-to-end
signing path only exercises on a tag push or `workflow_dispatch` run, since it
requires the workflow OIDC identity and a push to `ghcr.io`.
**Target Platform**: `ubuntu-24.04` GitHub-hosted runner.
**Project Type**: CI/release pipeline change (no Go code).
**Constraints**: keyless only — no keys or secrets added; existing release
outputs unchanged (OCI chart push is additive); release must fail if any
in-scope artifact fails to sign or attest.
**Scale/Scope**: 1 workflow file, 1 devbox dependency, 3 signed+attested
artifacts (2 images + 1 chart).

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. API-First | Pass | No API/CRD changes. |
| II. Handler-per-Provider | Pass | No handler code touched. |
| III. Library-First | Pass | No Go code touched. |
| IV. Tests Required | Pass | Verification via `actionlint`, `release-snapshot`, and documented `cosign`/`gh attestation` verify commands. No Go behaviour changes, so no Go tests apply. |
| V. Code Style | Pass | YAML/bash only; no narrating comments. |
| VI. Dependency Management | Pass | No `go.mod` change. One devbox CLI (`cosign`) added with justification. |
| VII. Handler Version Safety | Pass | No files under `pkg/handlers/*/mutation/` touched; zero rollout impact. |
| VIII. Handler Documentation | Pass | No handler added/changed. Supply-chain verification docs are optional (see T7). |

No violations.

## Project Structure

### Documentation (this feature)

```text
.specify/features/NCN-115081/
├── plan.md                # This file
└── spec.md                # Feature specification
```

### Source Code (repository root)

```text
devbox.json                          # Add cosign
.github/workflows/release-tag.yml    # Permissions + sign + attest + OCI chart push
docs/content/                        # (optional) verification guide
```

**Structure Decision**: Single workflow file plus one devbox dependency.
Signing/attestation are post-build steps in the existing `release-tag` job so
they share the job's OIDC token and the already-performed `ghcr.io` docker
login.

## Approach Notes (research)

- **Why crane over `dist/artifacts.json`**: artifact records differ between the
  `ko` and `dockers`/`docker_manifests` mechanisms, and selecting the
  multi-arch *index* (not per-arch) digest is fiddly. `crane digest <ref>`
  returns the exact digest a consumer pulls, for any mechanism, after the
  images are already pushed by `make release`.
- **Why not goreleaser `docker_signs`**: it targets goreleaser's
  docker/manifest artifacts and does not reliably cover the `ko`-built image,
  and it cannot sign the OCI chart (pushed by `helm`, outside goreleaser).
  Doing all signing uniformly in the workflow avoids two divergent code paths.
- **Why oras for the chart, and why a flat `-chart` path**: `helm push` always
  appends the `Chart.yaml` name as the final repo segment, so it can only
  produce `ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`
  (which collides with the image repo) or a `charts/` subpath. To publish the
  chart at the flat, image-distinct path
  `ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart`
  (which also maps cleanly onto Docker Hub's flat namespace if productised
  later) without renaming the chart, push the chart `.tgz` with `oras` using
  Helm's OCI media types. `helm pull` resolves by reference, not by name, so the
  pushed artifact remains `helm pull`-able while `Chart.yaml` is untouched.
- **Auth**: `helm`, `oras`, `crane`, and `cosign` all read
  `~/.docker/config.json`, which the existing `docker/login-action` step
  populates for `ghcr.io`. No extra login needed.
- **Keyless**: cosign v2 keyless is default (no `COSIGN_EXPERIMENTAL`). The job
  needs `id-token: write`; `actions/attest-build-provenance` additionally needs
  `attestations: write` and `push-to-registry: true` needs `packages: write`
  (already present).

## Tasks

Atomic and ordered. Do not proceed past a task until its acceptance check
passes.

### T1 — Add cosign and oras to devbox

**File**: `devbox.json`

Add `"cosign": "latest"` and `"oras": "latest"` to `packages` (keep the
existing alphabetical ordering of the map).

**Acceptance**: `devbox run -- cosign version` prints a v2.x version and
`devbox run -- oras version` prints a version.

### T2 — Grant OIDC + attestation permissions

**File**: `.github/workflows/release-tag.yml`

Extend the top-level `permissions` block:

```yaml
permissions:
  contents: write
  packages: write
  actions: write
  id-token: write
  attestations: write
```

**Acceptance**: `devbox run -- actionlint .github/workflows/release-tag.yml`
passes.

### T3 — Push Helm chart to ghcr.io as OCI (flat path, via oras)

**File**: `.github/workflows/release-tag.yml`

Extend the existing "Package Helm chart and attach to release" step (which runs
`helm package` then `gh release upload`) to also push the packaged `.tgz` to
the flat OCI path using `oras` with Helm's OCI media types. Keep the
`gh release upload` and do not change the gh-pages index flow.

The job sets `defaults.run.shell: bash`, which GitHub runs as
`bash --noprofile --norc -eo pipefail {0}` — so `errexit` and `pipefail` are
already on; no explicit `set` is needed.

```bash
VERSION=${{ github.ref_name }}
tgz=cluster-api-runtime-extensions-nutanix-${VERSION}.tgz
chart_repo=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart

# Helm's OCI config blob is the chart metadata as JSON.
devbox run -- helm show chart "${tgz}" | devbox run -- yq -o=json '.' >chart-config.json

devbox run -- oras push "${chart_repo}:${VERSION}" \
  --config chart-config.json:application/vnd.cncf.helm.config.v1+json \
  "${tgz}:application/vnd.cncf.helm.chart.content.v1.tar+gzip"
```

The layer is pushed with the title annotation defaulting to the file name
(`<chart>-<version>.tgz`), matching what `helm pull` expects.

**Acceptance**: workflow lints clean. On a dispatch run (T8),
`helm pull oci://${chart_repo} --version ${VERSION}` succeeds and the pulled
chart's `Chart.yaml` name is still `cluster-api-runtime-extensions-nutanix`.

### T4 — Resolve digests for all three artifacts

**File**: `.github/workflows/release-tag.yml`

Add a step (after `make release` and the chart push) that resolves the index
digest of each pushed reference with `crane` and exposes them as step outputs:

```yaml
- name: Resolve artifact digests
  id: digests
  env:
    VERSION: ${{ github.ref_name }}
  run: |
    caren=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix
    bundle=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer
    chart=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart
    # Assign per-line so a crane failure aborts the step: the default
    # `bash -eo pipefail` does not abort on a failure inside a command
    # substitution embedded in a larger command (e.g. echo "x=$(...)").
    caren_digest=$(devbox run -- crane digest "${caren}:${VERSION}")
    bundle_digest=$(devbox run -- crane digest "${bundle}:${VERSION}")
    chart_digest=$(devbox run -- crane digest "${chart}:${VERSION}")
    {
      echo "caren=${caren_digest}"
      echo "bundle=${bundle_digest}"
      echo "chart=${chart_digest}"
    } >>"${GITHUB_OUTPUT}"
```

**Acceptance**: on a dispatch run (T8) each output is a `sha256:` digest.

### T5 — cosign keyless sign images and chart

**File**: `.github/workflows/release-tag.yml`

Add a step that signs all three by digest:

```yaml
- name: Sign images and chart (cosign keyless)
  env:
    VERSION: ${{ github.ref_name }}
  run: |
    caren=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix
    bundle=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer
    chart=ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart
    devbox run -- cosign sign --yes "${caren}@${{ steps.digests.outputs.caren }}"
    devbox run -- cosign sign --yes "${bundle}@${{ steps.digests.outputs.bundle }}"
    devbox run -- cosign sign --yes "${chart}@${{ steps.digests.outputs.chart }}"
```

The default `bash -eo pipefail` (from `defaults.run.shell: bash`) makes any
signing failure fail the job (FR-009); each `cosign sign` is its own simple
command, so `errexit` aborts on the first failure.

**Acceptance**: on a dispatch run (T8), `cosign verify` succeeds for all three.

### T6 — Provenance attestations (images + chart)

**File**: `.github/workflows/release-tag.yml`

Add three `actions/attest-build-provenance@v3` steps (one per artifact), each
with `push-to-registry: true`:

```yaml
- name: Attest CAREN image
  uses: actions/attest-build-provenance@v3
  with:
    subject-name: ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix
    subject-digest: ${{ steps.digests.outputs.caren }}
    push-to-registry: true

- name: Attest bundle-initializer image
  uses: actions/attest-build-provenance@v3
  with:
    subject-name: ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer
    subject-digest: ${{ steps.digests.outputs.bundle }}
    push-to-registry: true

- name: Attest Helm chart (OCI)
  uses: actions/attest-build-provenance@v3
  with:
    subject-name: ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart
    subject-digest: ${{ steps.digests.outputs.chart }}
    push-to-registry: true
```

**Acceptance**: on a dispatch run (T8), `gh attestation verify` succeeds for
all three (by tag and by digest).

### T7 — (Optional) Verification docs

**File**: `docs/content/` (supply-chain / verification page)

Document the `cosign verify` and `gh attestation verify` commands consumers run
for the images and the OCI chart, including the certificate-identity regex for
the `release-tag.yml` workflow and the OIDC issuer.

**Acceptance**: markdown lints clean per `.cursor/rules/markdown-quality.mdc`.

### T8 — End-to-end verification on a pre-release dispatch

Run the workflow via `workflow_dispatch` against a pre-release tag (or a
disposable tag) and verify every artifact:

```bash
ident='^https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/\.github/workflows/release-tag.yml@.*'
issuer='https://token.actions.githubusercontent.com'
repo='nutanix-cloud-native/cluster-api-runtime-extensions-nutanix'

for ref in \
  ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix \
  ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-helm-chart-bundle-initializer \
  ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart ; do
  cosign verify --certificate-identity-regexp "${ident}" \
                --certificate-oidc-issuer "${issuer}" "${ref}:${TAG}"
  gh attestation verify "oci://${ref}:${TAG}" --repo "${repo}"
done

helm pull oci://ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart --version "${TAG}"
```

**Acceptance**: all `cosign verify`, `gh attestation verify`, and `helm pull`
commands succeed (maps to SC-001, SC-002, SC-003).

### T9 — Snapshot sanity + lint

Confirm goreleaser still builds and the workflow lints.

```bash
devbox run -- actionlint
devbox run -- make release-snapshot
```

**Acceptance**: both succeed; release outputs (images, chart `.tgz`, gh-pages
index inputs, example manifests, `caren-images.txt`) are unchanged versus
`origin/main` aside from the additive OCI chart push (SC-003, SC-004, SC-005).

## Complexity Tracking

No constitution violations. No complexity worth tracking.
