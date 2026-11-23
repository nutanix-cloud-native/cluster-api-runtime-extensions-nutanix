<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
-->

# MetalLB Configuration Controller Implementation Plan

**Goal**: Decouple MetalLB `IPAddressPool` and `L2Advertisement` application from the synchronous `AfterControlPlaneInitialized` / `BeforeClusterUpgrade` HTTP hook into a dedicated controller watching `Cluster` and `HelmChartProxy`.

**Jira Ticket**: [NCN-112141](https://jira.nutanix.com/browse/NCN-112141), [NCN-111113](https://jira.nutanix.com/browse/NCN-111113)
**Spec**: [./spec.md](./spec.md)

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| I. API-First | Pass | Uses existing `v1alpha1.ServiceLoadBalancer` and external CAAPH/MetalLB types. |
| II. Handler-per-Provider | Pass | Generic addon lifecycle handler + controller under `pkg/controllers/metallbconfig/`. |
| III. Library-First | Pass | Controller logic in `pkg/controllers/metallbconfig/`, configuration applier in `pkg/handlers/lifecycle/serviceloadbalancer/metallb/`, thin wiring in `cmd/main.go`. |
| IV. Tests Required | Pass | Unit tests (`testify`) + `envtest` integration tests (`gomega`). |
| V. Code Style | Pass | Enforced via `make fmt` and `make lint`. |
| VI. Dependency Management | Pass | Zero `go.mod` changes. |
| VII. Handler Version Safety | Pass | No topology mutation handler changes. |
| VIII. Handler Documentation | Pass | Updates `docs/content/addons/serviceloadbalancer.md`. |

## Tasks

- [x] **T1**: Add failing unit and `envtest` integration tests for `metallb.ApplyConfiguration`, `MetalLB.Apply` non-blocking behavior, and `metallbconfig.Reconciler`.
- [x] **T2**: Implement `HelmChartProxyName` and `ApplyConfiguration` in `pkg/handlers/lifecycle/serviceloadbalancer/metallb/configuration.go` and simplify `MetalLB.Apply` in `handler.go`.
- [x] **T3**: Implement `pkg/controllers/metallbconfig` (`doc.go`, `flags.go`, `controller.go`) and wire it into `cmd/main.go`.
- [x] **T4**: Clean up stale sequential-hook comment in `pkg/handlers/lifecycle/handlers.go`, add compile-time `BeforeClusterUpgrade` interface assertion in `pkg/handlers/lifecycle/serviceloadbalancer/handler.go`, and update `docs/content/addons/serviceloadbalancer.md`.
- [x] **T5**: Run `make go-generate`, `make test`, `make lint`, and `make pre-commit` checks.
- [x] **T6**: Cap `metallb-config` controller workqueue retry delay at `10s` (with unit test coverage) so transient `metallb-controller` webhook unavailability while worker nodes / CRS CNI start never triggers multi-minute backoff.
