---
title: "etcd"
---

Override the container image registry and tag for [etcd](https://github.com/etcd-io/etcd).

To enable this handler set the `etcdpatch` and `etcdvars` external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: image-registry
      external:
        generateExtension: "etcdpatch.capi-runtime-extensions"
        discoverVariablesExtension: "etcdvars.capi-runtime-extensions"
```

On the cluster resource then specify desired etcd image registry and/or image tag values:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: etcd
        values:
          imageRepository: my-registry.io/my-org/my-repo
          imageTag: "v3.5.99_custom.0"
```

Applying this configuration will result in the following value being set:

- KubeadmControlPlaneTemplate:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          etcd:
            local:
              imageRepository: "my-registry.io/my-org/my-repo"
              imageTag: "v3.5.99_custom.0"
    ```
