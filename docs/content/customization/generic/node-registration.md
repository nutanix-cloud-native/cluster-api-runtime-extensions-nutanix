+++
title = "Node registration configuration"
+++

Below is a list of node registration configuration options that can be set for `kubeadm init` and `kubeadm join`.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

### ignorePreflightErrors

#### Control plane

To configure `ignorePreflightErrors` for the control plane nodes, specify the following configuration:

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
            nodeRegistration:
              ignorePreflightErrors:
                - SystemVerification
```

Applying this configuration will result in the following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        initConfiguration:
          nodeRegistration:
            nodeRegistration:
              ignorePreflightErrors:
                - SystemVerification
        joinConfiguration:
            nodeRegistration:
              ignorePreflightErrors:
                - SystemVerification
    ```

#### Worker node

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
              nodeRegistration:
                ignorePreflightErrors:
                  - SystemVerification
```

Applying this configuration will result in the following value being set:

- `KubeadmConfigTemplate`:

  - ```yaml
    spec:
      joinConfiguration:
        nodeRegistration:
          ignorePreflightErrors:
            - SystemVerification
    ```
