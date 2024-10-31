+++
title = "etcd"
+++

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

The DNS configuration can then be manipulated via the cluster variables.
If the `dns` property is not specified, then the customization will be skipped.

## CoreDNS

The CoreDNS configuration can then be manipulated via the cluster variables.
If the `dns.coreDNS` property is not specified, then the customization will be skipped.

### Example

To change the repository and tag for the container image for the CoreDNS pod, specify the following configuration:

> Note do not include "coredns" in the repository, kubeadm already appends it.

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
          dns:
            coreDNS:
              image:
                repository: my-registry.io/my-org/my-repo
                tag: "v1.11.3_custom.0"
              updateStrategy: Manual
```

Applying this configuration will result in the following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          dns:
            imageRepository: "my-registry.io/my-org/my-repo"
            imageTag: "v1.11.3_custom.0"
    ```

The CoreDNS version can also be updated automatically. To do this, set the `updateStrategy` to `Automatic`:

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
          dns:
            coreDNS:
              updateStrategy: Automatic
```

Alternatively since the default value is `Automatic`, the following configuration is equivalent:

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
          dns:
            coreDNS: {}
```

Applying this configuration will result in the following value being set,
with the version of the CoreDNS image being set based on the cluster's Kubernetes version:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          dns:
            imageTag: "v1.11.3"
    ```
