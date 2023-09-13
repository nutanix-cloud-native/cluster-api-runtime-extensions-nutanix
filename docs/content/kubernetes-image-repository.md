---
title: "Kubernete Image Repository"
---

Override the container image repository used when pulling Kubernetes images.

To enable this handler set the `imagerepositorypatch` and `imagerepositoryvars` external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: image-repository
      external:
        generateExtension: "imagerepositorypatch.capi-runtime-extensions"
        discoverVariablesExtension: "imagerepositoryvars.capi-runtime-extensions"
```

On the cluster resource then specify desired Kubernetes image repository value:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: kubernetesImageRepository
        value: "my-registry.io/my-org/my-repo"
```

Applying this configuration will result in the following value being set:

- KubeadmControlPlaneTemplate:
  - `/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/imageRepository: my-registry.io/my-org/my-repo`
