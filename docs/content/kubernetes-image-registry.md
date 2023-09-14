---
title: "Kubernete Image Registry"
---

Override the container image registry used when pulling Kubernetes images.

To enable this handler set the `imageregistrypatch` and `imageregistryvars` external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: image-registry
      external:
        generateExtension: "imageregistrypatch.capi-runtime-extensions"
        discoverVariablesExtension: "imageregistryvars.capi-runtime-extensions"
```

On the cluster resource then specify desired Kubernetes image registry value:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: kubernetesImageRegistry
        value: "my-registry.io/my-org/my-repo"
```

Applying this configuration will result in the following value being set:

- KubeadmControlPlaneTemplate:
  - `/spec/template/spec/kubeadmConfigSpec/clusterConfiguration/imageRepository: my-registry.io/my-org/my-repo`
