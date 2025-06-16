+++
title = "kube-proxy mode"
+++

This customization allows configuration of the `kube-proxy` proxy mode. Currently, only `iptables` or `Disabled` modes
are supported. `Disabled` is useful when deploying a CNI implementation that can replace `kube-proxy` to avoid
potential conflicts. By default, `kube-proxy` is enabled in `iptables` mode.

## Example

To disable the deployment of `kube-proxy`, specify the following configuration:

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
          kubeProxy:
            mode: Disabled
```

Applying this configuration will result in the following configuration being applied:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          kubeadmConfigSpec:
            initConfiguration:
              skipPhases:
                - addon/kube-proxy
    ```
