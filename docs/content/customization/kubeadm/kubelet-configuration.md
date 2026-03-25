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
```
