---
title: "Calico CNI"
---

When deploying a cluster with CAPI, deployment and configuration of CNI is up to the user. By leveraging CAPI cluster
lifecycle hooks, this handler deploys Calico CNI on the new cluster via `ClusterResourceSets` at the
`AfterControlPlaneInitialized` phase.

Deployment of Calico is opt-in using the following configuration for the lifecycle hook to perform any actions.
The hook creates two `ClusterResourceSets`: one to deploy the Tigera Operator, and one to deploy
Calico via the Tigera `Installation` CRD. The Tigera Operator CRS is shared between all clusters in the operator,
whereas the Calico installation CRS is unique per cluster.

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

On the cluster resource then specify this `cni` value:

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
            cni:
              provider: calico
```

As ClusterResourceSets must exist in the same name as the cluster they apply to, the lifecycle hook copies default
ConfigMaps from the same namespace as the CAPI runtime extensions hook pod is running in. This enables users to
configure defaults specific for their environment rather than compiling the defaults into the binary.

The Helm chart comes with default configurations for the Calico Installation CRS per supported provider, but overriding
is possible. To do so, specify:

```bash
--set-file handlers.CalicoCNI.defaultInstallationConfigMaps.DockerCluster.configMap.content=<file>
```
