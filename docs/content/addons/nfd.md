+++
title = "Node Feature Discovery"
icon = "fa-solid fa-eye"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Node Feature Discovery] (NFD) on the new cluster at
the `AfterControlPlaneInitialized` phase.

Deployment of NFD is opt-in via the [provider-specific cluster configuration]({{< ref ".." >}}).

The hook uses either the [Cluster API Add-on Provider for Helm] or `ClusterResourceSet` to deploy the NFD resources
depending on the selected deployment strategy.

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
            nfd:
              strategy: HelmAddon
```

To deploy the addon via `ClusterResourceSet` replace the value of `strategy` with `ClusterResourceSet`.

## Pod Security Admission

NFD's master and worker DaemonSets require privileged pod features (hostNetwork,
host filesystem access, etc.) and would be rejected on clusters that enforce a
`baseline` or stricter Pod Security Standard. CAREN therefore labels the
`node-feature-discovery` namespace with `pod-security.kubernetes.io/enforce=privileged`
on every reconcile. This applies to both deployment strategies and means NFD
keeps working when an operator opts in to PSA enforcement.

[Node Feature Discovery]: https://github.com/kubernetes-sigs/node-feature-discovery
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
