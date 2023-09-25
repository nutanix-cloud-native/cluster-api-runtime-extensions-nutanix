+++
title = "HTTP proxy"
+++

In some network environments it is necessary to use HTTP proxy to successfuly execute HTTP requests.
This customization will configure Kubernetes components (`containerd`, `kubelet`) with appropriate configuration for
control plane and worker nodes, utilising systemd drop-ins to configure the necessary environment variables.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To configure HTTP proxy values, specify the following configuration:

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
          proxy:
            http: http://example.com
            https: http://example.com
            additionalNo:
              - no-proxy-1.example.com
              - no-proxy-2.example.com
```

The `additionalNo` list will be added to default pre-calculated values that apply on k8s networking
`localhost,127.0.0.1,<POD CIDRS>,<SERVICE CIDRS>,kubernetes,kubernetes.default,.svc,.svc.cluster.local`, plus
provider-specific addresses as required.

Applying this configuration will result in the following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          files:
            - path: "/etc/systemd/system/containerd.service.d/http-proxy.conf"
              content: <generated>
            - path: "/etc/systemd/system/kubelet.service.d/http-proxy.conf"
              content: <generated>
    ```

- `KubeadmConfigTemplate`:

  - ```yaml
    spec:
      files:
        - path: "/etc/systemd/system/containerd.service.d/http-proxy.conf"
          content: <generated>
        - path: "/etc/systemd/system/kubelet.service.d/http-proxy.conf"
          content: <generated>
    ```

Applying this configuration will result in new bootstrap files on the `KubeadmControlPlaneTemplate`
and `KubeadmConfigTemplate`.
