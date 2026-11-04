<!--
Copyright 2026 Nutanix. All rights reserved.
SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Nutanix auth secret overlay selection for generated examples

**Jira Ticket**: [NCN-114902](https://jira.nutanix.com/browse/NCN-114902)
**Feature Branch**: `NCN-114902-nutanix-auth-secret-overlays`
**Created**: 2026-07-02
**Status**: Draft
**Input**: User description: "Make similar kustomize overlay and patch changes in cluster-api-runtime-extensions-nutanix for setting auth secret, instead of env variable substitution"

## User Scenarios & Testing

### User Story 1 - Generate Nutanix examples with basic auth by default path (Priority: P1)

As an operator generating CAREN Nutanix quick-start examples, I want the generated
Prism Central credentials secret to use a basic-auth payload when username/password
are provided, so the manifest works without manual edits.

**Why this priority**: Existing users already depend on username/password and this
must remain supported.

**Independent Test**: Build the nutanix example via kustomize basic-auth overlay
and verify the `*-pc-creds` secret uses `type: basic_auth` with only
`username/password` under `prismCentral`.

**Acceptance Scenarios**:

1. **Given** `NUTANIX_USER` and `NUTANIX_PASSWORD` are set, **When** Nutanix
   examples are generated, **Then** the Prism Central credentials secret contains
   `type: "basic_auth"` and no `apiKey` field
2. **Given** both username/password and API key are set, **When** examples are
   generated, **Then** basic auth output is selected

### User Story 2 - Generate Nutanix examples with API key auth (Priority: P1)

As an operator using API key auth, I want generated Nutanix example manifests to
emit an API-key Prism Central credentials secret shape.

**Why this priority**: This is the new functionality requested in the ticket.

**Independent Test**: Build the nutanix example via api-key overlay and verify the
`*-pc-creds` secret uses `type: api_key` with only `apiKey` under `prismCentral`.

**Acceptance Scenarios**:

1. **Given** API key auth is selected, **When** Nutanix examples are generated,
   **Then** the Prism Central credentials secret contains `type: "api_key"` and
   `prismCentral.apiKey`
2. **Given** API key auth is selected, **When** examples are generated, **Then**
   no `username/password` fields appear in the Prism Central credentials JSON

### Edge Cases

- Overlay path selection points to a missing directory; generation must fail early
  and clearly rather than producing invalid manifests
- Username/password unset in API key mode must not break substitution for
  placeholders in generated output
- Existing non-Nutanix example generation must be unaffected

## Requirements

### Functional Requirements

- **FR-001**: The repository MUST provide Kustomize overlays for Nutanix example
  generation with two variants: `basic-auth` and `api-key`
- **FR-002**: The `basic-auth` overlay MUST patch the `${CLUSTER_NAME}-pc-creds`
  secret payload to contain only basic-auth Prism Central fields
- **FR-003**: The `api-key` overlay MUST patch the `${CLUSTER_NAME}-pc-creds`
  secret payload to contain only API key Prism Central fields
- **FR-004**: Nutanix example generation workflow MUST auto-derive auth mode from
  available env vars, preferring basic auth when both methods are present
- **FR-005**: The workflow MUST not require explicit user input for
  `NUTANIX_CREDENTIALS_TYPE` in normal usage
- **FR-006**: Docs and e2e config variables MUST include `NUTANIX_API_KEY` and
  explain auth precedence

### Key Entities

- **Nutanix example overlays**: Kustomize overlay directories used by
  `hack/examples/sync.sh` to generate final example manifests
- **Prism credentials secret**: `${CLUSTER_NAME}-pc-creds` secret containing the
  structured credentials JSON consumed by CAPX and addon handlers

## Success Criteria

### Measurable Outcomes

- **SC-001**: Running `kustomize build` on Nutanix basic-auth overlays produces
  manifests where `*-pc-creds` is basic-auth only
- **SC-002**: Running `kustomize build` on Nutanix api-key overlays produces
  manifests where `*-pc-creds` is API-key only
- **SC-003**: `hack/examples/sync.sh` completes and regenerates Nutanix examples
  from the selected overlay without affecting AWS/Docker/EKS generation
