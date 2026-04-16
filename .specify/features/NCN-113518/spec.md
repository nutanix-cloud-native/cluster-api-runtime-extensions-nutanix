<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# NCN-113518: Add enforceNodeAllocatable to CAREN kubelet configuration API

## Problem

CAREN exposes `systemReserved` and `kubeReserved` in the `KubeletConfiguration` API, but
without `enforceNodeAllocatable` these reservations only affect **scheduling**: the kubelet
subtracts them from node capacity to calculate `Allocatable`, which the scheduler uses to
decide how many pods fit on the node. However, nothing prevents system daemons or the
kubelet itself from consuming more than the declared reservation at runtime. If a system
process spikes beyond its reservation, it can starve pods of resources. Users need the
ability to add **runtime enforcement** of reservations via cgroups.

Enabling enforcement requires three kubelet settings:

1. `enforceNodeAllocatable` — which reservation types to enforce.
2. `systemReservedCgroup` — the cgroup path for OS daemons (required when enforcing
  `system-reserved`).
3. `kubeReservedCgroup` — the cgroup path for Kubernetes components (required when
  enforcing `kube-reserved`).

Exposing all three fields forces users to understand cgroup internals. On all
CAREN-supported OSes (Ubuntu, RHEL, Flatcar) the systemd cgroup paths are well-known and
identical: `/system.slice` for system services and `/system.slice/kubelet.service` for the
kubelet.

## Solution

Add a single `enforceNodeAllocatable` field to `KubeletConfiguration`. When the user
includes `system-reserved` or `kube-reserved` in the list, CAREN automatically injects the
well-known systemd cgroup paths into the rendered kubelet configuration patch. No cgroup
knowledge is required from the user.

## API

### New type

```go
type EnforceNodeAllocatableOption string

const (
    EnforceNodeAllocatablePods                       EnforceNodeAllocatableOption = "pods"
    EnforceNodeAllocatableSystemReserved             EnforceNodeAllocatableOption = "system-reserved"
    EnforceNodeAllocatableKubeReserved               EnforceNodeAllocatableOption = "kube-reserved"
    EnforceNodeAllocatableSystemReservedCompressible EnforceNodeAllocatableOption = "system-reserved-compressible"
    EnforceNodeAllocatableKubeReservedCompressible   EnforceNodeAllocatableOption = "kube-reserved-compressible"
)
```

### New field on `KubeletConfiguration`

```go
// EnforceNodeAllocatable specifies which resource types are enforced via cgroups.
// When "system-reserved" is included, the kubelet enforces systemReserved limits
// using the well-known systemd cgroup /system.slice. When "kube-reserved" is
// included, the kubelet enforces kubeReserved limits using
// /system.slice/kubelet.service. Default kubelet behaviour (when this field is
// not set) is to enforce only pods.
// +kubebuilder:validation:Optional
// +kubebuilder:validation:MaxItems=3
// +kubebuilder:validation:UniqueItems=true
// +kubebuilder:validation:items:Enum=pods;system-reserved;kube-reserved;system-reserved-compressible;kube-reserved-compressible
EnforceNodeAllocatable []EnforceNodeAllocatableOption `json:"enforceNodeAllocatable,omitempty"`
```

## Behaviour


| Field value                                    | Rendered kubelet config additions                                                     |
| ---------------------------------------------- | ------------------------------------------------------------------------------------- |
| nil / empty                                    | Nothing — identical to current behaviour                                              |
| `["pods"]`                                     | `enforceNodeAllocatable: ["pods"]`                                                    |
| `["pods", "system-reserved"]`                  | `enforceNodeAllocatable: [...]` + `systemReservedCgroup: /system.slice`               |
| `["pods", "kube-reserved"]`                    | `enforceNodeAllocatable: [...]` + `kubeReservedCgroup: /system.slice/kubelet.service` |
| `["pods", "system-reserved", "kube-reserved"]` | All three fields                                                                      |
| `["pods", "system-reserved-compressible"]`     | `enforceNodeAllocatable: [...]` + `systemReservedCgroup: /system.slice`               |
| `["pods", "kube-reserved-compressible"]`       | `enforceNodeAllocatable: [...]` + `kubeReservedCgroup: /system.slice/kubelet.service` |


Values in the rendered `enforceNodeAllocatable` list are sorted alphabetically for
idempotent output.

## Compressible variants (Kubernetes v1.32+)

The `-compressible` variants (`system-reserved-compressible`, `kube-reserved-compressible`)
enforce only compressible resources (CPU) via cgroups. This is the recommended starting
point for enabling enforcement because CPU is throttlable, whereas memory enforcement
requires OOM-killing.

## Mutual exclusivity (CEL validation)

`system-reserved` and `system-reserved-compressible` are mutually exclusive, as are
`kube-reserved` and `kube-reserved-compressible`. This is enforced at the API level via CEL
validation rules on the `KubeletConfiguration` struct, matching upstream kubelet validation.
MaxItems remains 3 because mutual exclusivity prevents more than one system variant and one
kube variant from coexisting.

## Scope

- Applies to both control plane nodes (`clusterConfig.controlPlane.kubeletConfiguration`)
and worker nodes (`workerConfig.kubeletConfiguration`).
- EKS is excluded (EKS does not use kubeadm).
- Fully opt-in: nil/empty field = no change = no-op for existing clusters.

## Handler version safety

No handler version bump is required. When the field is not set the template emits nothing
new, so patch output for existing clusters is identical.

## Well-known cgroup paths


| Enforcement target | Cgroup path                     | Rationale                                      |
| ------------------ | ------------------------------- | ---------------------------------------------- |
| `system-reserved`  | `/system.slice`                 | Standard systemd slice for all system services |
| `kube-reserved`    | `/system.slice/kubelet.service` | Where kubelet runs under systemd               |


## Documentation

Extend `docs/content/customization/kubeadm/kubelet-configuration.md` with the new field,
its allowed values, and examples for both control plane and worker nodes.

## Out of scope

- `reservedSystemCPUs` (CPU pinning for system use).
- Custom cgroup path overrides.
- `enforceNodeAllocatable` for EKS clusters.
