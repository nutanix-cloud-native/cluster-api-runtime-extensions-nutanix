+++
title = "Kubelet Configuration"
+++

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

Kubelet configuration is supported for:

- Control plane nodes via `clusterConfig.controlPlane.kubeletConfiguration`
- Worker nodes via `workerConfig.kubeletConfiguration`

There is no cluster-level default; control plane and worker settings are configured independently.

All fields are optional. When a field is not set, the kubelet default applies and no
patch is emitted for that field.

For full upstream documentation on each setting, see the
[KubeletConfiguration reference](https://kubernetes.io/docs/reference/config-api/kubelet-config.v1beta1/).

## Supported options

| Field | Type | Description |
|-------|------|-------------|
| `maxPods` | integer (50–256) | Maximum number of pods per node. |
| `systemReserved` | map of `cpu`, `memory`, `ephemeral-storage`, `pid` to quantities | Resources reserved for OS system daemons. |
| `kubeReserved` | map of `cpu`, `memory`, `ephemeral-storage`, `pid` to quantities | Resources reserved for Kubernetes components. |
| `evictionHard` | map of signal names to thresholds | Hard eviction thresholds (immediate pod eviction). |
| `evictionSoft` | map of signal names to thresholds | Soft eviction thresholds (eviction after grace period). |
| `evictionSoftGracePeriod` | map of signal names to durations | Grace periods for soft eviction signals. Keys must match `evictionSoft`. |
| `protectKernelDefaults` | boolean | Causes the kubelet to error if kernel flags differ from expected values. |
| `topologyManagerPolicy` | `none`, `best-effort`, `restricted`, `single-numa-node` | NUMA-aware resource alignment policy. |
| `cpuManagerPolicy` | `none`, `static` | Controls cpuset assignment. `static` enables exclusive CPU pinning for Guaranteed QoS pods. |
| `memoryManagerPolicy` | `None`, `Static` | Controls memory management. `Static` enables NUMA-aware memory allocation for Guaranteed QoS pods. |
| `podPidsLimit` | integer (1024–16384) | Maximum number of PIDs per pod. |
| `containerLogMaxSize` | quantity (e.g. `"10Mi"`) | Maximum size of a container log file before rotation. |
| `containerLogMaxFiles` | integer (≥2) | Maximum number of rotated log files per container. |
| `imageGCHighThresholdPercent` | integer (0–100) | Disk usage percent above which image GC always runs. Must be > `imageGCLowThresholdPercent`. |
| `imageGCLowThresholdPercent` | integer (0–100) | Disk usage percent below which image GC never runs. |
| `maxParallelImagePulls` | integer (≥0) | Maximum concurrent image pulls. When > 0, `serializeImagePulls` is automatically set to `false`. |
| `shutdownGracePeriod` | duration (e.g. `"30s"`) | Total time the node delays shutdown for pod termination. |
| `shutdownGracePeriodCriticalPods` | duration (e.g. `"10s"`) | Time reserved for terminating critical pods during shutdown. Must be ≤ `shutdownGracePeriod`. |
| `seccompDefault` | boolean | Apply the runtime's default seccomp profile (`RuntimeDefault`) to pods that do not specify one. See [Default seccomp profile](#default-seccomp-profile). |
| `enforceNodeAllocatable` | list of `pods`, `system-reserved`, `kube-reserved` | Which resource reservations are enforced via cgroups. See [Enforce node allocatable](#enforce-node-allocatable). |

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

## Examples

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

### Resource reservations

`systemReserved` and `kubeReserved` accept a map with keys `cpu`, `memory`,
`ephemeral-storage`, and `pid`. Values are Kubernetes resource quantities.

```yaml
kubeletConfiguration:
  systemReserved:
    cpu: "500m"
    memory: "1Gi"
    ephemeral-storage: "10Gi"
  kubeReserved:
    cpu: "200m"
    memory: "512Mi"
```

### Eviction thresholds

`evictionHard` and `evictionSoft` accept a map with signal names as keys and thresholds
(absolute quantities or percentages) as values. Valid signal names are `memory.available`,
`nodefs.available`, `nodefs.inodesFree`, `imagefs.available`, `imagefs.inodesFree`, and
`pid.available`.

When using `evictionSoft`, you must also set `evictionSoftGracePeriod` with matching keys.

```yaml
kubeletConfiguration:
  evictionHard:
    memory.available: "100Mi"
    nodefs.available: "10%"
    imagefs.available: "15%"
  evictionSoft:
    memory.available: "200Mi"
    nodefs.available: "15%"
  evictionSoftGracePeriod:
    memory.available: "30s"
    nodefs.available: "1m0s"
```

### Graceful node shutdown

`shutdownGracePeriod` sets the total time the node delays shutdown for pod termination.
`shutdownGracePeriodCriticalPods` sets the portion of that time reserved for critical pods
and must be less than or equal to `shutdownGracePeriod`.

```yaml
kubeletConfiguration:
  shutdownGracePeriod: "60s"
  shutdownGracePeriodCriticalPods: "15s"
```

### Image garbage collection

`imageGCHighThresholdPercent` must be greater than `imageGCLowThresholdPercent` when both
are set.

```yaml
kubeletConfiguration:
  imageGCHighThresholdPercent: 85
  imageGCLowThresholdPercent: 70
```

### Container log rotation

```yaml
kubeletConfiguration:
  containerLogMaxSize: "50Mi"
  containerLogMaxFiles: 10
```

### NUMA-aware topology management

For workloads sensitive to hardware topology (GPU, HPC, telco), you can combine
`topologyManagerPolicy`, `cpuManagerPolicy`, and `memoryManagerPolicy`.

```yaml
kubeletConfiguration:
  topologyManagerPolicy: single-numa-node
  cpuManagerPolicy: static
  memoryManagerPolicy: Static
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
