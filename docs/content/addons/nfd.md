+++
title = "Node Feature Discovery"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Node Feature
Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) (NFD) on the new cluster via `ClusterResourceSets`
at the `AfterControlPlaneInitialized` phase.

Deployment of NFD is opt-in via the  [provider-specific cluster configuration]({{< ref ".." >}}).

The hook creates a `ClusterResourceSet` to deploy the NFD resources.

## Example

To enable deployment of NFD on a cluster, specify the following values:

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
            nfd: {}
```
