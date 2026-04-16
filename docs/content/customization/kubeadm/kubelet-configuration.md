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
- `enforceNodeAllocatable`

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

## Enforce node allocatable

By default, `systemReserved` and `kubeReserved` only affect **scheduling**: the kubelet
subtracts them from the node's capacity to calculate the `Allocatable` value that the
scheduler sees. However, nothing prevents system daemons or the kubelet itself from
consuming more than the declared reservation. If a system process spikes beyond its
reservation, it can starve pods of resources.

The `enforceNodeAllocatable` field adds **runtime enforcement** by creating cgroups that
cap the reserved processes to their declared limits. Accepted values are:

| Value | Enforces | Since K8s |
|-------|----------|-----------|
| `pods` | Pod resource limits | v1.0 |
| `system-reserved` | All system-reserved resources (CPU + memory) | v1.6 |
| `kube-reserved` | All kube-reserved resources (CPU + memory) | v1.6 |
| `system-reserved-compressible` | Only compressible (CPU) system-reserved resources | v1.32 |
| `kube-reserved-compressible` | Only compressible (CPU) kube-reserved resources | v1.32 |

The `-compressible` variants are the **recommended starting point** for enabling
enforcement. They enforce only CPU (which is throttlable) and skip memory (which requires
OOM-killing). This matches the upstream Kubernetes recommendation and is the default in
OpenShift 4.22+.

`system-reserved` and `system-reserved-compressible` are **mutually exclusive**, as are
`kube-reserved` and `kube-reserved-compressible`. The maximum number of items is 3 (one
system variant, one kube variant, and `pods`).

When any system-reserved variant is included, CAREN automatically configures the well-known
systemd cgroup path `/system.slice` for enforcement. When any kube-reserved variant is
included, CAREN configures `/system.slice/kubelet.service`. You do not need to specify
cgroup paths.

This field is optional. When not set, the kubelet default behaviour (`pods` only) applies
and no changes are made to existing clusters.

### Example: compressible-only enforcement (recommended)

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
              systemReserved:
                cpu: "500m"
                memory: "1Gi"
              kubeReserved:
                cpu: "200m"
                memory: "512Mi"
              enforceNodeAllocatable:
                - pods
                - system-reserved-compressible
                - kube-reserved-compressible
```

### Example: full enforcement (CPU + memory)

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
              systemReserved:
                cpu: "500m"
                memory: "1Gi"
              kubeReserved:
                cpu: "200m"
                memory: "512Mi"
              enforceNodeAllocatable:
                - pods
                - system-reserved
                - kube-reserved
```
