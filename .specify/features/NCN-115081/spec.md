<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Keyless signing and provenance attestation for release artifacts

**Jira Ticket**: [NCN-115081](https://jira.nutanix.com/browse/NCN-115081)
**Feature Branch**: `NCN-115081-keyless-signing-attestation`
**Created**: 2026-06-10
**Status**: Draft
**Input**: User description: "i would like to sign helm chart and images using GH actions keyless attestation"

## User Scenarios & Testing

### User Story 1 - Consumer verifies image provenance before deploying (Priority: P1)

A platform operator pulling a CAREN container image from `ghcr.io` wants to
confirm the image was built by the official CAREN release workflow and has not
been tampered with, without trusting any long-lived public key.

**Why this priority**: Provenance is the core ask. It is the strongest single
guarantee a consumer can check and is the basis for admission policies (e.g.
Kyverno/Sigstore policy-controller) that gate deployment on verified builds.

**Independent Test**: After a release, run
`gh attestation verify oci://ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix:<tag> --repo nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`
and confirm it reports a verified SLSA provenance attestation tied to the
release workflow's OIDC identity.

**Acceptance Scenarios**:

1. **Given** a published release, **When** a consumer runs
   `gh attestation verify` against either CAREN image by tag or digest,
   **Then** verification succeeds and reports the workflow identity
   (`.../release-tag.yml`) and the `token.actions.githubusercontent.com`
   issuer.
2. **Given** a tampered or unofficial image with the same tag in a different
   registry, **When** a consumer runs the same verification, **Then**
   verification fails (no matching attestation).

### User Story 2 - Consumer verifies a cosign signature on images and chart (Priority: P1)

A consumer whose tooling standardises on cosign wants to verify a keyless
cosign signature on both container images and the OCI Helm chart using the
workflow's OIDC identity.

**Why this priority**: The user explicitly asked for both cosign signatures
and provenance attestations. cosign `verify` is widely integrated into
existing admission controllers and CI gates.

**Independent Test**: Run
`cosign verify --certificate-identity-regexp '^https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/\.github/workflows/release-tag.yml@.*' --certificate-oidc-issuer https://token.actions.githubusercontent.com <ref>@<digest>`
against each image and the OCI chart, and confirm success.

**Acceptance Scenarios**:

1. **Given** a published release, **When** a consumer runs `cosign verify`
   with the correct certificate identity and OIDC issuer against either image,
   **Then** verification succeeds.
2. **Given** the chart pushed to `ghcr.io` as an OCI artifact, **When** a
   consumer runs `cosign verify` against the chart's OCI reference with the
   correct identity and issuer, **Then** verification succeeds.

### User Story 3 - Consumer pulls and verifies the chart from ghcr.io (Priority: P1)

A consumer who installs CAREN via Helm wants to pull the chart directly from
`ghcr.io` as an OCI artifact and verify its provenance, getting the same
guarantee as for the images with the attestation co-located in the registry.

**Why this priority**: The user explicitly requested that the chart be pushed
to `ghcr.io`. Registry-colocated provenance is the strongest verification path
for the chart and matches the image flow.

**Independent Test**: Run
`helm pull oci://ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart --version <tag>`
and
`gh attestation verify oci://ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart:<tag> --repo nutanix-cloud-native/cluster-api-runtime-extensions-nutanix`;
confirm a verified provenance attestation.

**Acceptance Scenarios**:

1. **Given** a published release, **When** a consumer runs `helm pull
   oci://...` for the chart at the release version, **Then** the chart is
   pulled successfully from `ghcr.io`.
2. **Given** the chart pushed to `ghcr.io`, **When** a consumer runs
   `gh attestation verify` against the chart's OCI reference, **Then**
   verification succeeds and reports the `release-tag.yml` workflow identity.

### Edge Cases

- The release workflow is run via `workflow_dispatch` (manual) rather than a
  tag push. Signing and attestation MUST still work because the OIDC identity
  is the same workflow.
- A re-run of the release for the same tag re-publishes images at the same
  digests. Re-signing/re-attesting MUST be idempotent or non-fatal (existing
  signatures/attestations must not cause the job to fail).
- The `ko`-built CAREN image and the buildx-built bundle-initializer image are
  produced by different goreleaser mechanisms; both MUST end up signed and
  attested even if they require different steps.
- The multi-arch manifest list (index) digest, not just per-architecture image
  digests, MUST be covered so that consumers verifying the tag they actually
  pull get a result.
- A fork running the release workflow MUST NOT be able to produce signatures
  that verify against the upstream identity (guaranteed by OIDC identity
  binding; no shared keys exist).

## Requirements

### Functional Requirements

- **FR-001**: The release workflow MUST sign both published container images
  (`cluster-api-runtime-extensions-nutanix` and
  `cluster-api-runtime-extensions-helm-chart-bundle-initializer`) with cosign
  using keyless (Sigstore/Fulcio/Rekor) signing driven by the GitHub Actions
  OIDC token. No signing keys or secrets may be introduced.
- **FR-002**: The release workflow MUST push the packaged Helm chart to
  `ghcr.io` as an OCI artifact (e.g.
  `oci://ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart`)
  at the release version, in addition to the existing `.tgz` release asset and
  GitHub Pages Helm index.
- **FR-003**: The release workflow MUST sign the OCI Helm chart with cosign
  keyless, addressed by digest.
- **FR-004**: The release workflow MUST generate a SLSA build provenance
  attestation for both container images and for the OCI Helm chart via
  `actions/attest-build-provenance`, pushed to the registry as a referrer
  (`push-to-registry: true`).
- **FR-005**: Provenance and signature subjects for images and the OCI chart
  MUST be addressed by digest, and for the images MUST include the multi-arch
  manifest (index) digest.
- **FR-006**: The release job MUST grant the minimum additional permissions
  required: `id-token: write` and `attestations: write`, in addition to the
  existing `contents: write`, `packages: write`, `actions: write`.
- **FR-007**: The change MUST NOT remove or alter the existing release outputs
  — the same images, chart `.tgz` release asset, GitHub Pages Helm index, and
  example manifests MUST continue to be produced. The OCI chart push is
  additive.
- **FR-008**: cosign MUST be provided through the project's devbox environment
  (added to `devbox.json`); no host-level installation may be assumed.
- **FR-009**: The signing/attestation steps MUST fail the release if signing
  or attestation fails for a real (non-snapshot) release, so an unsigned
  release cannot be published silently.

### Key Entities

- **CAREN image** (`ghcr.io/.../cluster-api-runtime-extensions-nutanix`):
  multi-arch, `ko`-built. Needs cosign signature + provenance attestation.
- **Bundle-initializer image**
  (`ghcr.io/.../cluster-api-runtime-extensions-helm-chart-bundle-initializer`):
  multi-arch, buildx-built. Needs cosign signature + provenance attestation.
- **Helm chart**: published two ways — the existing `.tgz` GitHub release
  asset (consumed via the GitHub Pages Helm index, unchanged) and a new OCI
  artifact at
  `ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart`.
  The OCI artifact is the signed + attested chart (cosign signature +
  provenance, both by digest).
- **OIDC identity**: the `release-tag.yml` workflow identity at
  `token.actions.githubusercontent.com`; the trust anchor for all
  verification.

## Success Criteria

### Measurable Outcomes

- **SC-001**: `gh attestation verify` succeeds for both images and for the OCI
  chart (by tag and by digest), each reporting the `release-tag.yml` workflow
  identity and the GitHub Actions OIDC issuer.
- **SC-002**: `cosign verify` succeeds for both images and the OCI chart,
  using the workflow certificate identity and
  `https://token.actions.githubusercontent.com` issuer.
- **SC-003**: `helm pull oci://ghcr.io/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix-chart --version <tag>`
  succeeds, and the set of GitHub release assets (images, chart `.tgz`,
  metadata, example manifests, `caren-images.txt`) and the GitHub Pages Helm
  index are unchanged.
- **SC-004**: No long-lived signing key or secret is added to the repository,
  the workflow, or the organisation; verification depends only on the public
  Sigstore transparency log and the workflow OIDC identity.
- **SC-005**: A release run that fails to sign or attest any in-scope artifact
  fails the workflow (no partially-signed release is published).
