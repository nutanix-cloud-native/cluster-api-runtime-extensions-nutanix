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

## Scale from zero

CAREN deploys Cluster Autoscaler with appropriate permissions to enable scaling nodepools from zero. However, CAPI
providers must implement functionality as described in the [autoscaling from zero proposal][Scale from zero status
updates] in order for scaling from zero to be possible.

For those providers that have not implemented this (e.g. Docker, Nutanix), scaling from zero is still possible by
providing annotations on the `MachineDeployments` to allow Cluster Autoscaler to make appropriate scaling decisions.
The following example shows the required annotations to add to:

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
              # The initial MachineDeployment will have 0 Machines
              cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "3"
              cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "0"
              capacity.cluster-autoscaler.kubernetes.io/cpu: "8"
              capacity.cluster-autoscaler.kubernetes.io/memory: "8112564Ki"
          name: scale-from-zero-example
          # Do not set the replicas field, otherwise the topology controller will revert back the autoscaler's changes
```

If the nodepool is labelled and/or tainted, additional annotations are required in order for Cluster Autoscaler to take
these labels and taints into account to scale nodepools that have node affinity and/or tolerations configured:

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
              # The initial MachineDeployment will have 0 Machines
              cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "3"
              cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "0"
              capacity.cluster-autoscaler.kubernetes.io/cpu: "8"
              capacity.cluster-autoscaler.kubernetes.io/memory: "8112564Ki"
              capacity.cluster-autoscaler.kubernetes.io/labels: "node-restriction.kubernetes.io/my-app="
              capacity.cluster-autoscaler.kubernetes.io/taints: "mytaint=tainted:NoSchedule"
          name: scale-from-zero-example
          # Do not set the replicas field, otherwise the topology controller will revert back the autoscaler's changes
```

[Cluster Autoscaler]: https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler/cloudprovider/clusterapi
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
[Pivot]: https://main.cluster-api.sigs.k8s.io/clusterctl/commands/move#pivot
[Scale from zero status updates]: https://github.com/kubernetes-sigs/cluster-api/blob/v1.10.4/docs/proposals/20210310-opt-in-autoscaling-from-zero.md#infrastructure-machine-template-status-updates
