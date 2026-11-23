<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# Lifecycle Hook Timeouts Implementation Plan

**Goal**: Set `TimeoutSeconds: 30` (with a `25s` server-side handler context deadline) on lifecycle runtime extension hooks in `common/pkg/server`, and align internal addon waiter timeouts (`20s`) below CAPI's `30s` HTTP client deadline.

**Jira Ticket**: [NCN-111113](https://jira.nutanix.com/browse/NCN-111113), [NCN-112141](https://jira.nutanix.com/browse/NCN-112141)
**Spec**: [./spec.md](./spec.md)

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| I. API-First | Pass | No CRD/API schema changes. |
| II. Handler-per-Provider | Pass | Shared server wiring in `common/pkg/server`, lifecycle waiter constants in `pkg/handlers/lifecycle/`. |
| III. Library-First | Pass | Implemented in `common/pkg/server` and `pkg/handlers/lifecycle/`. |
| IV. Tests Required | Pass | Unit tests in `common/pkg/server`, `pkg/handlers/lifecycle/addons`, and `pkg/handlers/lifecycle/cni/cilium`. |
| V. Code Style | Pass | Enforced via `make fmt` and `make lint`. |
| VI. Dependency Management | Pass | Zero `go.mod` changes. |
| VII. Handler Version Safety | Pass | No topology mutation handler changes. |
| VIII. Handler Documentation | Pass | Internal runtime hook timeout tuning; no user-facing YAML changes. |

## Tasks

- [x] **T1**: Write failing tests in `common/pkg/server/server_test.go`, `pkg/handlers/lifecycle/addons/helmaddon_test.go`, and `pkg/handlers/lifecycle/cni/cilium/template_test.go`.
- [x] **T2**: Implement `LifecycleHookTimeoutSeconds = 30` and `LifecycleHookHandlerTimeout = 25 * time.Second` in `common/pkg/server/server.go`.
- [x] **T3**: Align `defaultWaitTimeout = 20 * time.Second` (`defaultWaitInterval = 2 * time.Second`) in `pkg/handlers/lifecycle/addons/helmaddon.go` and `waitTimeout = 20 * time.Second` in `pkg/handlers/lifecycle/cni/cilium/handler.go`.
- [x] **T4**: Run `make test` and `make lint` across `common` and root modules.
