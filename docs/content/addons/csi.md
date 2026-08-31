+++
title = "CSI"
icon = "fa-solid fa-hard-drive"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys a requested CSI provider on the new cluster at the
`AfterControlPlaneInitialized` phase.

Deployment of CSI is opt-in via the [provider-specific cluster configuration]({{< ref ".." >}}).

The hook uses the [Cluster API Add-on Provider for Helm] to deploy the CSI driver. On Nutanix clusters the default
provider is `nutanix`.

## Volume snapshots

When `addons.csi.snapshotController` is set, CAREN deploys the CSI snapshot-controller and its `v1` CRDs.

The Nutanix CSI Helm chart creates `nutanix-snapshot-class` only after
`snapshot.storage.k8s.io/v1` is served on the workload cluster. The chart would otherwise
fall back to `snapshot.storage.k8s.io/v1beta1`, which snapshot-controller 5.x does not install,
and Helm install would fail.

The lifecycle hook retries until the snapshot `v1` API exists, then enables
`createVolumeSnapshotClass` so Helm renders the class as `v1`.

## Example

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
            csi:
              defaultStorage:
                provider: nutanix
                storageClassConfig: volume
              providers:
                nutanix:
                  strategy: HelmAddon
              snapshotController:
                strategy: HelmAddon
```

[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
