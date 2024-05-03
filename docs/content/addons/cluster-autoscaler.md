+++
title = "Cluster Autoscaler"
icon = "fa-solid fa-up-right-and-down-left-from-center"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Cluster Autoscaler] on the management cluster
for every Cluster at the `AfterControlPlaneInitialized` phase.Unlike other addons, the Cluster Autoscaler
is deployed on the management cluster because it also interacts with the CAPI resources to scale the number of Machines.
The Cluster Autoscaler Pod will not start on the management cluster until the CAPI resources are [pivoted][Pivot]
to that management cluster.

> Note the Cluster Autoscale controller needs to be running for any scaling operations to occur,
> just updating the min and max size annotations in the Cluster object will not be enough.
> You can however manually change the number of replicas by modifying the MachineDeployment object directly.

Deployment of Cluster Autoscaler is opt-in via the  [provider-specific cluster configuration]({{< ref ".." >}}).

The hook uses either the [Cluster API Add-on Provider for Helm] or `ClusterResourceSet` to deploy the cluster-autoscaler
resources depending on the selected deployment strategy.

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
              strategy: HelmAddon
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
          # Do not set the replicas field, otherwise the topology controller will revert back the autoscaler's changes
```

To deploy the addon via `ClusterResourceSet` replace the value of `strategy` with `ClusterResourceSet`.

[Cluster Autoscaler]: https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/cloudprovider/clusterapi
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
[Pivot]: https://main.cluster-api.sigs.k8s.io/clusterctl/commands/move#pivot
