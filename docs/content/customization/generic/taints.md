+++
title = "Tainting nodes"
+++

Tainting nodes prevents pods from being scheduled on them unless they explicitly tolerate the taints applied to the
nodes. See the [Kubernetes Taints and Tolerations] documentation for more details.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To configure taints for the control plane nodes, specify the following configuration:

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
            taints:
              - key: some-key
                effect: NoSchedule
                value: some-value
```

Taints for individual nodepools can be configured similarly:

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
              taints:
                - key: some-key
                  effect: NoSchedule
                  value: some-value
```

Applying this configuration will result in the following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        initConfiguration:
          nodeRegistration:
            taints:
              - key: some-key
                effect: NoSchedule
                value: some-value
        joinConfiguration:
          nodeRegistration:
            taints:
              - key: some-key
                effect: NoSchedule
                value: some-value
    ```

- `KubeadmConfigTemplate`:

  - ```yaml
    spec:
      joinConfiguration:
        nodeRegistration:
          taints:
            - key: some-key
              effect: NoSchedule
              value: some-value
    ```

[Kubernetes Taints and Tolerations]: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
