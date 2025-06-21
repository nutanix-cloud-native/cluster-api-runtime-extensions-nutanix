+++
title = "kube-proxy mode"
+++

This customization allows configuration of the `kube-proxy` proxy mode. Currently, only `iptables` and `nftables`
modes are supported. By default, `kube-proxy` is enabled in `iptables` mode by `kubeadm`.

## Examples

### Enabling nftables kube-proxy mode

Enabling `nftables` is done via the following configuration:

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
            mode: nftables
```

Applying this configuration will result in the following configuration being applied to create a
`KubeProxyConfiguration` and append it to the kubeadm configuration that is used when `kubeadm init`
is executed:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          kubeadmConfigSpec:
            files:
              - path: "/etc/kubernetes/kubeproxy-config.yaml"
                owner: "root:root"
                permissions: "0644"
                content: |-
                  ---
                  apiVersion: kubeproxy.config.k8s.io/v1alpha1
                  kind: KubeProxyConfiguration
                  mode: nftables
          preKubeadmCommands:
            - /bin/sh -ec 'cat /etc/kubernetes/kubeproxy-config.yaml >> /run/kubeadm/kubeadm.yaml'
    ```

### Skipping kube-proxy installation

To disable the deployment and upgrade of `kube-proxy`, specify the following configuration:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    controlPlane:
      metadata:
        annotations:
          controlplane.cluster.x-k8s.io/skip-kube-proxy: ""
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
