+++
title = "Registry"
icon = "fa-solid fa-eye"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys an OCI [Distribution] registry,
at the `AfterControlPlaneInitialized` phase and configures it as a mirror on the new cluster.
The registry will be deployed as a StatefulSet with a persistent volume claim for storage
and multiple replicas for high availability.
A sidecar container in each Pod running [Regsync] will periodically sync the OCI artifacts across all replicas.

Deployment of this registry is opt-in via the [provider-specific cluster configuration]({{< ref ".." >}}).

The hook will use the [Cluster API Add-on Provider for Helm] to deploy the registry resources.

## Example

To enable deployment of the registry on a cluster, specify the following values:

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
            registry: {}
```

[Distribution]: https://github.com/distribution/distribution
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
[Regsync]: https://regclient.org/usage/regsync/
