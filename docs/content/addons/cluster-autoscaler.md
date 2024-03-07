+++
title = "Cluster Autoscaler"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Cluster Autoscaler][cluster-autoscaler]
on the new cluster via `ClusterResourceSets` at the `AfterControlPlaneInitialized` phase.

Deployment of Cluster Autoscaler is opt-in via the  [provider-specific cluster configuration]({{< ref ".." >}}).

The hook creates a `ClusterResourceSet` to deploy the Cluster Autoscaler resources.

## Example

To enable deployment of Cluster Autoscaler on a cluster, specify the following values:

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
            clusterAutoscaler:
              strategy: ClusterResourceSet
    workers:
      machineDeployments:
        - class: default-worker
          metadata:
            annotations:
              # Set the following annotations to configure the Cluster Autoscaler
              # The initial MachineDeployment will have 1 Machine
              cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "3"
              cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "1"
          name: md-0
          # Remove the replicas field, otherwise the topology controller will revert back the autoscaler's changes
```

[cluster-autoscaler]: https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/cloudprovider/clusterapi
