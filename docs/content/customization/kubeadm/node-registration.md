+++
title = "Node registration configuration"
+++

Below is a list of node registration configuration options that can be set for `kubeadm init` and `kubeadm join`.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

### ignorePreflightErrors

Kubeadm runs preflight checks to ensure the machine is compatible with Kubernetes and its dependencies.
The `SystemVerification` check is known to result in false positives.
For example, it fails when the Linux Kernel version is not supported by kubeadm,
even if the kernel has all the required features.
For this reason, we skip the check by default.

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

`ignorePreflightErrors` for individual nodepools can be configured similarly:

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

By default, the following value will be set for both control plane and worker nodes:

```yaml
    variables:
      - name: clusterConfig
        value:
          controlPlane:
            nodeRegistration:
              ignorePreflightErrors:
                - SystemVerification
      - name: workerConfig
        value:
          nodeRegistration:
            ignorePreflightErrors:
              - SystemVerification
```

This can be enabled by setting `ignorePreflightErrors` to an empty list:

```yaml
    variables:
      - name: clusterConfig
        value:
          controlPlane:
            nodeRegistration:
              ignorePreflightErrors: []
      - name: workerConfig
        value:
          nodeRegistration:
            ignorePreflightErrors: []
```
