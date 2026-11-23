<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Feature Specification: Advertise 30s lifecycle hook timeout and align internal waiter budgets

**Jira Ticket**: [NCN-111113](https://jira.nutanix.com/browse/NCN-111113), [NCN-112141](https://jira.nutanix.com/browse/NCN-112141)
**Feature Branch**: `jimmi/lifecycle-hook-timeouts-5091`
**Created**: 2026-11-23
**Status**: Approved

## User Scenarios & Testing

### User Story 1 - Lifecycle hooks advertise Cluster API's maximum 30s timeout (Priority: P1)

When Cluster API queries CAREN's runtime extension `Discovery` endpoint, all lifecycle hook handlers (`BeforeClusterCreate`, `AfterControlPlaneInitialized`, `BeforeClusterUpgrade`, `AfterControlPlaneUpgrade`, `BeforeClusterDelete`) advertise `timeoutSeconds: 30` instead of leaving `timeoutSeconds` unset (which Cluster API defaults to `10s`). Topology mutation hooks (`DiscoverVariables`, `GeneratePatches`, `ValidateTopology`) retain the default `10s` timeout.

**Why this priority**: Lifecycle hooks execute up to 17 addon handlers in parallel and talk to both the management and workload cluster API servers; `10s` is too short when addon waiters are active. `30s` is the maximum `timeoutSeconds` permitted by Cluster API's `DiscoveryResponse` validator.

**Acceptance Scenarios**:

1. **Given** CAREN's runtime extension server has registered lifecycle and mutation handlers, **When** Cluster API calls `/hooks.runtime.cluster.x-k8s.io/v1alpha1/discovery`, **Then** every lifecycle handler advertises `timeoutSeconds: 30` and every mutation handler leaves `timeoutSeconds` unset (`nil`).

---

### User Story 2 - Lifecycle hooks return structured `Failure` responses before CAPI's 30s HTTP client deadline (Priority: P1)

When an internal waiter in a lifecycle handler does not succeed in time, CAREN's server-side handler context (`25s`) and per-resource waiter timeouts (`20s`) expire before Cluster API's `30s` HTTP client timeout cancels the request, allowing CAREN to return a structured HTTP `200 OK` JSON response with `status: Failure` and an actionable diagnostic message.

**Acceptance Scenarios**:

1. **Given** a lifecycle hook handler blocks until its `ctx` expires and writes `ResponseStatusFailure`, **When** called by an HTTP client with a longer client timeout, **Then** the server cancels the handler context first and returns a valid JSON `Failure` response with the handler's message rather than timing out the HTTP transport.
2. **Given** `addons.WithDefaultWaiter()` or Cilium's resource readiness waiters are invoked, **When** a resource does not become ready, **Then** their default poll timeouts (`20s`) are strictly less than Cluster API's `30s` hook timeout.

## Requirements

- **FR-001**: Set `TimeoutSeconds` to `30` on all 5 lifecycle `ExtensionHandler` registrations in `common/pkg/server/server.go`.
- **FR-002**: Wrap lifecycle hook `HandlerFunc` invocations in `common/pkg/server/server.go` with a `25s` context timeout so sequential operations within a single hook invocation return structured responses before CAPI's `30s` HTTP deadline.
- **FR-003**: Reduce `waitToBeReady` default poll timeout from `30s` to `20s` (and poll interval from `5s` to `2s`) in `pkg/handlers/lifecycle/addons/helmaddon.go`.
- **FR-004**: Reduce `waitTimeout` from `30s` to `20s` in `pkg/handlers/lifecycle/cni/cilium/handler.go`.
- **FR-005**: Do not alter any topology mutation handler `GeneratePatches` output.
