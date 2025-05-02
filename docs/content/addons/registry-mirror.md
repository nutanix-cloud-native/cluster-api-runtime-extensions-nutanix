+++
title = "Registry Mirror"
icon = "fa-solid fa-eye"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys an OCI [Distribution] (Distribution) registry
as a mirror on the new cluster at the `AfterControlPlaneInitialized` phase.

Deployment of registry mirror is opt-in via the [provider-specific cluster configuration]({{< ref ".." >}}).

The hook will use the [Cluster API Add-on Provider for Helm] to deploy the registry mirror resources.

## Example

To enable deployment of the registry mirror on a cluster, specify the following values:

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
            registryMirror: {}
```

[Distribution]: https://github.com/distribution/distribution
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
