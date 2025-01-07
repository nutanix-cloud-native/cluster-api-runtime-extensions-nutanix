+++
title = " Container Object Storage Interface (COSI)"
icon = "fa-solid fa-eye"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Container Object Storage Interface] (COSI)
on the new cluster at the `AfterControlPlaneInitialized` phase.

Deployment of COSI is opt-in via the [provider-specific cluster configuration]({{< ref ".." >}}).

The hook uses the [Cluster API Add-on Provider for Helm] to deploy the NFD resources.

## Example

To enable deployment of COSI on a cluster, specify the following values:

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
          addons:
            cosi: {}
```

[Container Object Storage Interface]: https://kubernetes.io/blog/2022/09/02/cosi-kubernetes-object-storage-management/
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
