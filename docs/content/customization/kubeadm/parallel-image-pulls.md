+++
title = "Parallel Image Pulls"
+++

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

The parallel image pull configuration can then be manipulated via the cluster variables.
If the `maxParallelImagePullsPerNode` property is not specified, then the default value of `1` will be used
which is equivalent to serialized image pulls.

Setting this value to `0` results in unlimited parallel image pulls.

### Example

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
          maxParallelImagePullsPerNodePerNode: 10
```

Applying this configuration will result in a `KubeletConfiguration` patch being added which will be
applied by `kubeadm` on `init` and `join`:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          kubeadmConfigSpec:
            files:
              - path: "/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json"
                owner: "root:root"
                permissions: "0644"
                content: |-
                  ---
                  apiVersion: kubelet.config.k8s.io/v1beta1
                  kind: KubeletConfiguration
                  serializeImagePulls: false
                  maxParallelImagePulls: 10
    ```

- `KubeadmConfigTemplate`

  - ```yaml
    spec:
      kubeadmConfigSpec:
        files:
          - path: "/etc/kubernetes/patches/kubeletconfigurationmaxparallelimagepulls+strategic.json"
            owner: "root:root"
            permissions: "0644"
            content: |-
              ---
              apiVersion: kubelet.config.k8s.io/v1beta1
              kind: KubeletConfiguration
              serializeImagePulls: false
              maxParallelImagePulls: 10
    ```
