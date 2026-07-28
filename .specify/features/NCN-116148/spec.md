<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Consolidate runtime hooks and admission webhooks onto one HTTPS server

**Jira Ticket**: [NCN-116148](https://jira.nutanix.com/browse/NCN-116148)
**Feature Branch**: `NCN-116148-consolidate-webhook-servers`
**Created**: 2026-07-24
**Status**: Draft
**Input**: User description: "consolidate the server for the runtime hooks with the admission webhooks, specifically to use the same port and tls cert" (option 2; refactor)

## User Scenarios & Testing

### User Story 1 - Single HTTPS listener serves both surfaces (Priority: P1)

An operator runs CAREN and expects both CAPI runtime extension hooks and Kubernetes admission webhooks to be served from one HTTPS listener in the manager process, on one port, with one TLS certificate.

**Why this priority**: This is the refactor goal. Today two listeners (9443 runtime hooks, 9444 admission) and two certs add chart and operational complexity without a protocol requirement.

**Independent Test**: Deploy the chart. Confirm the Deployment exposes a single HTTPS container port (9443). Confirm one TLS secret is mounted. Call runtime Discovery via the runtimehooks Service and an admission path via the admission Service; both succeed over TLS verified with the same CA.

**Acceptance Scenarios**:

1. **Given** CAREN is installed from the chart, **When** inspecting the manager container, **Then** there is exactly one HTTPS serving port (9443) and no 9444 container port.
2. **Given** CAREN is running, **When** Cluster API calls runtime Discovery through the `*-runtimehooks` Service, **Then** Discovery succeeds over TLS.
3. **Given** CAREN is running, **When** the API server calls an admission webhook (e.g. `/validate-cluster`) through the `*-admission` Service, **Then** admission succeeds over TLS using the same serving certificate CA as runtime hooks.

---

### User Story 2 - Stable Service names on upgrade (Priority: P1)

An operator upgrades CAREN. ExtensionConfig and admission webhook configurations keep their existing Service name references (`*-runtimehooks` and `*-admission`). Both Services target the shared container port.

**Why this priority**: Avoids clientConfig service-name churn and limits upgrade blast radius.

**Independent Test**: Diff rendered chart before/after. Service names unchanged; both `targetPort` values point at the same container port. ExtensionConfig `clientConfig.service.name` and webhook `clientConfig.service.name` values unchanged.

**Acceptance Scenarios**:

1. **Given** an existing install using `*-runtimehooks` and `*-admission` Services, **When** upgrading to this version, **Then** both Service names remain and both target the shared HTTPS port.
2. **Given** the upgrade, **When** inspecting ExtensionConfig and Mutating/ValidatingWebhookConfigurations, **Then** `clientConfig.service.name` values are unchanged.

---

### User Story 3 - Single Certificate and simplified flags (Priority: P1)

The chart issues one cert-manager Certificate (`*-runtimehooks-tls`) with DNS SANs for both Services. Admission webhook CA injection points at that Certificate. The `*-admission-tls` Certificate is removed. Process flags are `--webhook-port` and `--webhook-cert-dir` only; `--admission-webhook-cert-dir` is removed.

**Why this priority**: Completes consolidation of TLS and config surface.

**Independent Test**: Helm template/render shows one Certificate with both Service DNS names, no `*-admission-tls`, admission webhook annotations use `inject-ca-from` for `*-runtimehooks-tls`, and Deployment args omit `--admission-webhook-cert-dir`.

**Acceptance Scenarios**:

1. **Given** the chart is rendered, **When** listing Certificates, **Then** only `*-runtimehooks-tls` exists and its `dnsNames` include both `*-runtimehooks` and `*-admission` Service FQDNs.
2. **Given** the chart is rendered, **When** inspecting Mutating/ValidatingWebhookConfigurations, **Then** `cert-manager.io/inject-ca-from` references `*-runtimehooks-tls` (not `*-admission-tls`).
3. **Given** the manager binary, **When** listing flags, **Then** `--webhook-port` and `--webhook-cert-dir` exist and `--admission-webhook-cert-dir` does not.

---

### User Story 4 - Upgrade is a no-op for managed Clusters (Priority: P1)

Upgrading CAREN MUST NOT change topology mutation patch output or trigger Machine rollouts.

**Why this priority**: Constitution principle VII (handler version safety). This is infrastructure wiring only.

**Independent Test**: Compare `GeneratePatches` output for representative Clusters before and after; output must be identical. No mutation handler version bumps in the change.

**Acceptance Scenarios**:

1. **Given** existing managed Clusters, **When** CAREN is upgraded to include this refactor, **Then** no topology mutation handler names or patch outputs change.
2. **Given** the PR, **When** reviewing the diff, **Then** no new handler versions are introduced under `pkg/handlers/*/mutation/` solely due to this work.

### Edge Cases

- What happens when only one of the two Services is reachable? Each Service independently routes to the same pod port; failure of one Service does not stop the other from working if the pod is healthy.
- How does cert-manager CA re-injection behave when admission webhook annotations switch from `*-admission-tls` to `*-runtimehooks-tls`? Expect a brief period until inject-ca-from updates `caBundle`; both Services must already be covered by the Certificate SANs before traffic relies on the shared cert.
- What if an out-of-chart install still passes `--admission-webhook-cert-dir`? The flag is removed; the process must fail flag parsing (unknown flag) rather than silently ignore a second cert dir.
- Path collisions: admission paths (`/mutate-cluster`, `/validate-cluster`, `/mutate-addons`, `/preflight-cluster`) MUST remain distinct from runtime hook paths (`/hooks.runtime.cluster.x-k8s.io/...`).

## Requirements

### Functional Requirements

- **FR-001**: The manager process MUST serve CAPI runtime extension hooks and Kubernetes admission webhooks on a single HTTPS listener owned by the controller-runtime manager `WebhookServer`.
- **FR-002**: The default HTTPS listen port MUST be 9443. Port 9444 MUST NOT be used by the chart or default binary configuration.
- **FR-003**: Runtime extension handlers MUST be registered on the manager `WebhookServer` and MUST NOT start a second TLS listener.
- **FR-004**: Admission webhook registration paths and behavior MUST remain unchanged (`/mutate-cluster`, `/validate-cluster`, `/mutate-addons`, `/preflight-cluster`).
- **FR-005**: Runtime hook registration paths and handler behavior MUST remain unchanged (Discovery and all registered extension handlers).
- **FR-006**: The Helm chart MUST keep separate Services named `*-runtimehooks` and `*-admission`, both selecting the manager pods and targeting the shared HTTPS container port.
- **FR-007**: The Helm chart MUST issue a single cert-manager Certificate named `*-runtimehooks-tls` whose `dnsNames` include both Service DNS names (`*.svc` and `*.svc.cluster.local` for each).
- **FR-008**: The Helm chart MUST delete the `*-admission-tls` Certificate resource.
- **FR-009**: MutatingWebhookConfiguration and ValidatingWebhookConfiguration MUST set `cert-manager.io/inject-ca-from` to the namespace/`*-runtimehooks-tls` Certificate.
- **FR-010**: ExtensionConfig MUST continue to inject CA from `*-runtimehooks-tls` and reference the `*-runtimehooks` Service.
- **FR-011**: The Deployment MUST mount a single TLS secret (`*-runtimehooks-tls`), pass a single cert dir flag, and expose a single HTTPS container port.
- **FR-012**: CLI MUST expose `--webhook-port` and `--webhook-cert-dir` only. `--admission-webhook-cert-dir` MUST be removed (not aliased).
- **FR-013**: This change MUST NOT alter topology mutation `GeneratePatches` output or require a handler version bump.

### Key Entities

- **Manager WebhookServer**: Single controller-runtime HTTPS server serving both admission and runtime-extension HTTP paths.
- **Runtimehooks Service**: Stable ClusterIP/NodePort Service used by ExtensionConfig; targets shared HTTPS port.
- **Admission Service**: Stable ClusterIP/NodePort Service used by admission webhook configs; targets shared HTTPS port.
- **Runtimehooks TLS Certificate**: Sole cert-manager Certificate/secret for serving TLS; SANs cover both Services.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Rendered chart contains exactly one HTTPS container port and exactly one webhook TLS Certificate.
- **SC-002**: Both Services remain named as today and share the same `targetPort`.
- **SC-003**: Runtime Discovery and at least one admission webhook path succeed against a running install using the shared cert.
- **SC-004**: Representative `GeneratePatches` outputs are byte-identical before and after the change.
- **SC-005**: `--admission-webhook-cert-dir` is absent from `--help` / flag registration.

## Design Decisions

| Decision | Choice |
| --- | --- |
| Consolidation level | Shared port + TLS (option 2) |
| Go wiring | Register runtime handlers on manager `WebhookServer`; no second `Start`/listen |
| Services | Keep two Services, same target port |
| Flags | Single `--webhook-port` / `--webhook-cert-dir`; remove admission cert-dir flag |
| Certificate | Keep `*-runtimehooks-tls`; add admission SANs; remove `*-admission-tls` |
| Default port | 9443 |

## Out of Scope

- Upstream changes to CAPI `exp/runtime/server` to natively accept an injected `webhook.Server`
- Merging the two Kubernetes Services into one
- Changing admission or runtime handler business logic
- Metrics (8080) or health probe (8081) consolidation
