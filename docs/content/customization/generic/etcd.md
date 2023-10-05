+++
title = "etcd"
+++

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

The etcd configuration can then be manipulated via the cluster variables. If the `etcd` property is not specified, then
the customization will be skipped.

## Example

To change the repository and tag for the container image for the etcd pod, specify the following configuration:

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
          etcd:
            image:
              repository: my-registry.io/my-org/my-repo
              tag: "v3.5.99_custom.0"
```

Applying this configuration will result in the following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          etcd:
            local:
              imageRepository: "my-registry.io/my-org/my-repo"
              imageTag: "v3.5.99_custom.0"
    ```
