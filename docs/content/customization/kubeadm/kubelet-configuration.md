+++
title = "Kubelet Configuration"
+++

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

Kubelet configuration is supported for:

- Control plane nodes via `clusterConfig.controlPlane.kubeletConfiguration`
- Worker nodes via `workerConfig.kubeletConfiguration`

There is no cluster-level default; control plane and worker settings are configured independently.

## Supported options

The following fields are supported under `kubeletConfiguration`:

- `maxPods`
- `automaticReservations`
- `systemReserved`
- `kubeReserved`
- `evictionHard`
- `evictionSoft`
- `evictionSoftGracePeriod`
- `protectKernelDefaults`
- `topologyManagerPolicy`
- `cpuManagerPolicy`
- `memoryManagerPolicy`
- `podPidsLimit`
- `containerLogMaxSize`
- `containerLogMaxFiles`
- `imageGCHighThresholdPercent`
- `imageGCLowThresholdPercent`
- `maxParallelImagePulls`
- `shutdownGracePeriod`
- `shutdownGracePeriodCriticalPods`
- `seccompDefault`

## Automatic resource reservations

Instead of hand-picking `systemReserved`/`kubeReserved` per node size, you can opt in to
automatic, node-size-aware reservations. Each node computes its `kubeReserved` (CPU and memory)
and a hard eviction threshold at boot from its actual capacity — the same approach GKE and EKS
use.

`automaticReservations` is mutually exclusive with `systemReserved`, `kubeReserved`, and
`evictionHard`; setting it alongside any of them is rejected at admission. Other kubelet fields
(such as `maxPods`) can still be set.

The `CapacityTiered` profile reserves:

- CPU: 6% of the first core, 1% of the second, 0.5% of cores three and four, and 0.25% of each
  core beyond four.
- Memory: 255Mi below 1Gi total; otherwise 25% of the first 4Gi, 20% of the next 4Gi, 10% of the
  next 8Gi, 6% of the next 112Gi, and 2% of memory above 128Gi.
- A hard eviction threshold of `memory.available: 100Mi`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    workers:
      machineDeployments:
      - class: default-worker
        name: md-0
        variables:
          overrides:
          - name: workerConfig
            value:
              kubeletConfiguration:
                automaticReservations:
                  profile: CapacityTiered
```

## Default seccomp profile

`seccompDefault` instructs the kubelet to apply the container runtime's
`RuntimeDefault` seccomp profile to every pod that does not explicitly set
`spec.securityContext.seccompProfile` (or the equivalent on a container).
This provides a baseline syscall filter for unhardened workloads without
requiring per-pod changes.

Enabling `seccompDefault: true` on both control plane and worker
`kubeletConfiguration` mitigates Linux kernel local-privilege-escalation
issues that depend on syscalls excluded from `RuntimeDefault` (for example,
the Dirty Frag exploit chain CVE-2026-43284 / CVE-2026-43500, which relies
on `unshare`, `add_key`, and `keyctl`).

Caveats:

- Pods that opt out with `seccompProfile.type: Unconfined` are not constrained.
- Pods running with `privileged: true` or `CAP_SYS_ADMIN` are not constrained
  by seccomp.
- Workloads that legitimately require syscalls outside `RuntimeDefault` (for
  example, sandboxed runtimes, profiling agents, or some networking tools)
  may need a custom seccomp profile or `Unconfined`.
- Changing this value rolls the affected machines, since it is rendered into
  the `KubeadmConfig` and triggers a node template change.

## Example

### Control plane

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          controlPlane:
            kubeletConfiguration:
              maxPods: 200
              protectKernelDefaults: true
              seccompDefault: true
```

### Worker nodes

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    workers:
      machineDeployments:
      - class: default-worker
        name: md-0
        variables:
          overrides:
          - name: workerConfig
            value:
              kubeletConfiguration:
                maxPods: 250
                podPidsLimit: 4096
                seccompDefault: true
```
