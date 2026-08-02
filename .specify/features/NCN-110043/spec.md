

<!--
 Copyright 2026 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# Feature Specification: Cilium as an alternative ServiceLoadBalancer provider

**Jira Ticket**: [NCN-110043](https://jira.nutanix.com/browse/NCN-110043)
**Feature Branch**: `NCN-110043-cilium-service-load-balancer`
**Created**: 2026-04-17
**Status**: Draft
**Input**: User description: "i would like to add cilium as an alternative load balancer to metallb"

## Summary

CAREN currently supports a single `ServiceLoadBalancer` provider, MetalLB, which
handles IP allocation (`IPAddressPool`) and L2 announcement (`L2Advertisement`)
for Kubernetes `Service` objects of type `LoadBalancer`. This feature adds
**Cilium** as a second, selectable provider that reuses the Cilium CNI that
CAREN can already install, and layers its native LB IPAM + L2 announcements on
top so that a separate MetalLB install is not required.

Cilium-as-LB is only valid when Cilium is also the CNI, and when `kube-proxy`
is disabled (so Cilium's kube-proxy replacement is active — a hard prerequisite
for Cilium's L2 announcement feature).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Operator picks Cilium as the ServiceLoadBalancer provider (Priority: P1)

As a cluster operator who has already chosen Cilium as the CNI for a workload
cluster and disabled `kube-proxy`, I want to select `Cilium` as the
`ServiceLoadBalancer` provider so that I get `LoadBalancer`-type Services
working out of the box without adding MetalLB as a second, redundant dataplane.

**Why this priority**: This is the core feature. Without it, users who already
run Cilium + kube-proxy-replacement must install MetalLB purely to get
`Service.Type=LoadBalancer`, duplicating functionality and creating
configuration drift.

**Independent Test**: On a Cilium-CNI cluster with `kube-proxy` disabled, set
`addons.serviceLoadBalancer.provider=Cilium` with one `addressRanges` entry,
and verify a newly-created `Service` of type `LoadBalancer` is assigned an IP
from that range and is reachable from outside the cluster on the node's L2
segment.

**Acceptance Scenarios**:

1. **Given** a Cluster with `addons.cni.provider=Cilium`,
  `kubeProxy.mode=disabled`, and
   `addons.serviceLoadBalancer={provider: Cilium, configuration: {addressRanges: [...]}}`,
   **When** the cluster is created,
   **Then** the workload cluster has a `CiliumLoadBalancerIPPool` matching the
   configured ranges and a `CiliumL2AnnouncementPolicy` selecting all
   `LoadBalancer`-type Services, and no MetalLB resources are installed.
2. **Given** such a cluster and a `Service` of type `LoadBalancer`,
  **When** the Service is created,
   **Then** it is assigned an IP from the configured ranges and is reachable
   via ARP on the node network.
3. **Given** such a cluster, **When** the Cluster resource is reconciled
  again with no spec changes, **Then** no resources are rolled out or
   restarted (idempotency).

---

### User Story 2 - Operator is prevented from configuring Cilium LB in an unsupported combination (Priority: P1)

As a cluster operator, when I misconfigure a cluster by asking for
`ServiceLoadBalancer.provider=Cilium` without Cilium-CNI or without
`kube-proxy` disabled, I want to get an immediate, descriptive error so that
the cluster is never partially provisioned into a broken state.

**Why this priority**: Cilium-LB has hard prerequisites. Letting the handler
fail mid-flight would leave clusters in partial states; webhook-level
rejection is the best UX and must ship with the feature.

**Independent Test**: Create two Clusters that are each missing exactly one
prerequisite (CNI != Cilium; kube-proxy enabled). In both cases, admission must
fail with a message that names the missing prerequisite.

**Acceptance Scenarios**:

1. **Given** `addons.serviceLoadBalancer.provider=Cilium` and
  `addons.cni.provider=Calico`,
   **When** the Cluster is created or updated,
   **Then** the admission webhook rejects the request with an error that
   mentions both the selected LB provider and the CNI prerequisite.
2. **Given** `addons.serviceLoadBalancer.provider=Cilium`,
  `addons.cni.provider=Cilium`, and no `kubeProxy.mode=disabled`,
   **When** the Cluster is created or updated,
   **Then** the admission webhook rejects the request with an error that
   mentions the kube-proxy prerequisite.
3. **Given** the webhook is somehow bypassed (e.g. the skip annotation is
  used, or admission is disabled during a migration) and the cluster reaches
   the lifecycle handler, **When** the handler runs, **Then** it fails fast
   with the same descriptive error rather than installing partial resources.

---

### User Story 3 - Existing MetalLB users are unaffected by the change (Priority: P1)

As an existing CAREN user running MetalLB, I want the introduction of Cilium
as an LB provider to be a pure addition — no change in behaviour for my
clusters, no Machine rollouts on upgrade, and no new required fields.

**Why this priority**: CAREN's constitution (Principle VII) forbids silent
rollouts. MetalLB is the current default and is deployed in production.

**Independent Test**: Upgrade CAREN on a management cluster running a
MetalLB-configured workload cluster. Verify (a) no Machines are rolled out,
(b) no MetalLB resources are modified, (c) `HelmReleaseProxy` / helm
release generation is unchanged.

**Acceptance Scenarios**:

1. **Given** an existing Cluster with
  `addons.serviceLoadBalancer.provider=MetalLB`,
   **When** CAREN is upgraded to the version that introduces this feature,
   **Then** nothing about the Cluster or its Machines changes.
2. **Given** a Cluster spec that does not set
  `addons.serviceLoadBalancer`, **When** the cluster is reconciled under the
   new CAREN, **Then** neither MetalLB nor Cilium LB resources are deployed.

---

### User Story 4 - Nutanix operator catches a PC-in-range misconfiguration regardless of provider (Priority: P2)

As a Nutanix cluster operator, when my Prism Central IP accidentally falls
inside the LB `addressRanges`, I want the existing guard-rail to continue to
protect me whether I picked MetalLB or Cilium.

**Why this priority**: A known foot-gun that already caused production
incidents on MetalLB; it applies identically to Cilium.

**Independent Test**: Attempt to create a Nutanix Cluster with PC IP inside
the `addressRanges` for each of the two providers; both must be rejected with
the same error structure.

**Acceptance Scenarios**:

1. **Given** a Nutanix Cluster with PC IP `10.0.0.5` and
  `addons.serviceLoadBalancer={provider: Cilium, configuration: {addressRanges: [{start: 10.0.0.1, end: 10.0.0.20}]}}`,
   **When** the Cluster is admitted,
   **Then** it is rejected with a "Prism Central IP must not be part of…"
   error.
2. **Given** the same misconfiguration but with `provider=MetalLB`,
  **When** the Cluster is admitted, **Then** it is rejected with the same
   error (parity with existing behaviour).

### Edge Cases

- **Provider switch on a live cluster** (e.g. MetalLB → Cilium): out of scope
for this feature. The initial release treats provider changes as unsupported;
users must delete and recreate the cluster, or manually clean up the old
provider's resources. The webhook SHOULD reject such an update with a clear
message so users are not surprised.
- **Cluster has no `addons.cni` at all**: Cilium LB is rejected (CNI
prerequisite not met).
- **User sets `provider=Cilium` but omits `configuration.addressRanges`**:
Matches MetalLB behaviour — the provider is installed (in the Cilium case,
the CNI handler enables L2 announcements) but no `CiliumLoadBalancerIPPool`
is created, so `LoadBalancer` Services stay in `Pending` until the operator
creates a pool manually. This is allowed; the spec documents it.
- `**addressRanges` overlap across entries**: Out of scope. `CiliumLoadBalancerIPPool`
accepts multiple blocks; we pass them through as-is. Cilium itself
handles collisions.
- **Cilium chart upgrade changes the default pool behaviour**: Cilium 1.19.x
(the currently bundled version) supports LB IPAM + L2 announcements in their
stable form. No chart bump is in scope.
- **IPv6 / dual-stack ranges**: Out of scope for this feature, for API-shape
reasons only. The existing `AddressRange` type is IPv4-only
(`+kubebuilder:validation:Format=ipv4`) and is shared across all
`serviceLoadBalancer` providers; both MetalLB and Cilium can technically
handle IPv6 pools. Extending `AddressRange` to accept IPv6 is a separate
change that should be designed once and applied to every provider at the
same time, so it is tracked as follow-up work outside NCN-110043 rather
than bundled in.
- **Coexistence with `awsloadbalancercontroller` Ingress addon**: Unchanged.
That addon targets cloud LBs, orthogonal to ServiceLoadBalancer provider.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The `addons.serviceLoadBalancer.provider` enum MUST be extended
from `{MetalLB}` to `{MetalLB, Cilium}` in all infrastructure-specific
`ClusterConfig` CRDs (AWS, Docker, Nutanix, EKS) that expose it.
- **FR-002**: The shape of `configuration.addressRanges` MUST NOT change; it
continues to accept `[]{start, end}` IPv4 pairs with `MinItems=1`,
`MaxItems=10`.
- **FR-003**: When `serviceLoadBalancer.provider=Cilium`, the Cluster
admission webhook MUST reject the request if `addons.cni.provider` is not
`Cilium`, naming both the chosen LB provider and the CNI prerequisite in the
error message.
- **FR-004**: When `serviceLoadBalancer.provider=Cilium`, the Cluster
admission webhook MUST reject the request if `kubeProxy.mode` is not
`disabled`, naming the kube-proxy prerequisite in the error message.
- **FR-005**: The webhook MUST honour the same `preflight.cluster.caren.nutanix.com/skip`
escape hatch used elsewhere in the codebase, and Cilium-specific validation
MUST NOT run on `Delete` admission operations.
- **FR-006**: The Cilium CNI lifecycle handler MUST, when
`serviceLoadBalancer.provider=Cilium`, render Cilium Helm values with L2
announcement support enabled (i.e. `l2announcements.enabled=true` plus
reasonable `k8sClientRateLimit` defaults) in addition to the existing
`kubeProxyReplacement` wiring.
- **FR-007**: The Cilium CNI handler's Helm-values rendering MUST remain a
no-op (identical output) for clusters that do not set
`serviceLoadBalancer.provider=Cilium`, so that upgrading CAREN does not
trigger a Cilium rollout on existing clusters.
- **FR-008**: The `ServiceLoadBalancer` lifecycle handler MUST dispatch to a
new `cilium` sub-provider when `provider=Cilium`.
- **FR-009**: The `cilium` sub-provider MUST apply one
`CiliumLoadBalancerIPPool` on the workload cluster, containing one `blocks[]`
entry per configured `addressRange`, mapping `start`/`end` to Cilium's
`blocks[].start`/`blocks[].stop`. The object's name and namespace MUST be
deterministic so that repeated reconciles are idempotent; the specific name
is an implementation detail chosen by the plan, but it MUST NOT collide with
any existing well-known Cilium resource name.
- **FR-010**: The `cilium` sub-provider MUST apply one
`CiliumL2AnnouncementPolicy` on the workload cluster that announces
LoadBalancer IPs on all nodes (matching the behaviour of MetalLB's current
`L2Advertisement` default in CAREN).
- **FR-011**: The `cilium` sub-provider MUST NOT install or uninstall Cilium
itself; Cilium lifecycle remains entirely owned by the existing Cilium CNI
handler.
- **FR-012**: The `cilium` sub-provider MUST re-validate the Cilium-CNI and
kube-proxy prerequisites at runtime (defence-in-depth) and fail the
`AfterControlPlaneInitialized` / `BeforeClusterUpgrade` hook with a
descriptive message if they are not met.
- **FR-013**: The Nutanix webhook check that rejects a PC IP inside the
MetalLB `addressRanges` MUST be generalised to apply to any configured
`serviceLoadBalancer` provider that owns `addressRanges` (i.e. both MetalLB
and Cilium today).
- **FR-014**: Applying the Cilium LB resources MUST be idempotent — a
subsequent hook invocation with the same spec MUST produce server-side-apply
patches that are no-ops.
- **FR-015**: Cilium CRDs required by the handler (`CiliumLoadBalancerIPPool`,
`CiliumL2AnnouncementPolicy`) MUST be vendored into `api/external/` under
the existing pattern used for MetalLB (`api/external/go.universe.tf/metallb`),
so CAREN does not take a direct dependency on the upstream Cilium Go module.
- **FR-016**: User-facing documentation
(`docs/content/addons/serviceloadbalancer.md`) MUST be updated with a Cilium
example and a prerequisites section.
- **FR-017**: E2E tests MUST cover the Cilium happy path (`Service.Type=LoadBalancer`
reachable on a Cilium-CNI + kube-proxy-disabled cluster) at parity with the
MetalLB happy path.
- **FR-018**: BGP-based advertisement is explicitly out of scope. The design
and spec document that as a future extension.
- **FR-019**: This feature MUST NOT require a topology-mutation handler
version bump (Constitution Principle VII). `ServiceLoadBalancer` and
Cilium-CNI are lifecycle-hook-driven addons, not `pkg/handlers/*/mutation/`
patches. If the plan reveals that any mutation-handler patch output would
change for clusters that do not opt into Cilium-LB, that change MUST be
moved behind a new handler version per the constitution.

### Key Entities *(include if feature involves data)*

- `**ServiceLoadBalancer` (existing API)**: `{provider, configuration}`.
`provider` enum grows by `Cilium`. `configuration.addressRanges` unchanged.
- `**CiliumLoadBalancerIPPool` (new, remote)**: Cilium CRD that declares IP
ranges available to LoadBalancer Services. Equivalent concept to MetalLB's
`IPAddressPool`. The exact API group/version used by the handler MUST match
the GA/stable version shipped in the bundled Cilium chart; pinning the
version is an implementation-plan decision.
- `**CiliumL2AnnouncementPolicy` (new, remote)**: Cilium CRD that causes the
Cilium agent to ARP-announce LoadBalancer IPs on the node network. Equivalent
concept to MetalLB's `L2Advertisement`. API group/version pinned at
plan-time per the rule above.
- **Cilium Helm values ConfigMap (existing)**: grows a conditional block that
sets `l2announcements.enabled=true` and `k8sClientRateLimit` defaults when
the CNI handler determines Cilium is also the selected LB provider.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: On a Cilium-CNI cluster with kube-proxy disabled and
`serviceLoadBalancer.provider=Cilium` plus one `addressRanges` entry, a
newly-created `Service.Type=LoadBalancer` receives an IP from the configured
range and its `EXTERNAL-IP` field is populated within 60 seconds of Service
creation.
- **SC-002**: HTTP traffic to the assigned LB IP from an external client on
the same L2 segment succeeds (≥99% success over 10 sequential requests in
the e2e test).
- **SC-003**: 100% of Clusters that configure `provider=Cilium` without Cilium
CNI, or without `kubeProxy.mode=disabled`, are rejected at admission with a
single, human-readable error that names the missing prerequisite.
- **SC-004**: Zero existing MetalLB-configured Clusters experience a Machine
rollout, MetalLB resource modification, or HelmReleaseProxy generation bump
as a direct result of merging this feature.
- **SC-005**: A second reconcile of a Cilium-LB cluster with no spec change
produces no object mutations on the workload cluster (idempotency, verified
by server-side-apply dry-run diff in the unit/integration test).
- **SC-006**: The Nutanix PC-IP-in-range webhook check rejects misconfigured
Clusters with identical error semantics for both MetalLB and Cilium
providers (verified by parameterised webhook unit test).
