---
title: "Node Feature Discovery"
---

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Node Feature
Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) (NFD) on the new cluster via
`ClusterResourceSets` at the `AfterControlPlaneInitialized` phase.

Deployment of NFD is opt-in using the following configuration for the lifecycle hook to perform any actions. The hook
creates a `ClusterResourceSet` to deploy the NFD resources.

To enable the meta handler enable the `clusterconfigvars` and `clusterconfigpatch` external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: cluster-config
      external:
        generateExtension: "clusterconfigpatch.capi-runtime-extensions"
        discoverVariablesExtension: "clusterconfigvars.capi-runtime-extensions"
```

On the cluster resource then specify this `nfd` value:

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
