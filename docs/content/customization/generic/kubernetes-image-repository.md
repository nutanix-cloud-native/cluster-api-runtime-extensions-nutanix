+++
title = "Kubernetes Image Repository"
+++

Override the container image repository used when pulling Kubernetes images.

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
          kubernetesImageRepository: "my-registry.io/my-org/my-repo"
```

Applying this configuration will result in the following value being set:

- KubeadmControlPlaneTemplate:
  - `/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/imageRepository: my-registry.io/my-org/my-repo`
