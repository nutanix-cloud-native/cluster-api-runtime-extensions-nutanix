+++
title = "CNI"
icon = "fa-solid fa-network-wired"
+++

When deploying a cluster with CAPI, deployment and configuration of CNI is up to the user. By leveraging CAPI cluster
lifecycle hooks, this handler deploys a requested CNI provider on the new cluster at the `AfterControlPlaneInitialized`
phase.

The hook uses either the [Cluster API Add-on Provider for Helm] or `ClusterResourceSet` to deploy the CNI resources
depending on the selected deployment strategy.

Currently the hook supports [Cilium](#cilium) and [Calico](#calico) CNI providers.

## Cilium

Deployment of Cilium is opt-in via the  [provider-specific cluster configuration]({{< ref ".." >}}).

### Cilium Example

To enable deployment of Cilium on a cluster, specify the following values:

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
              provider: Cilium
              strategy: HelmAddon
```

## Cilium Example With Custom Values

To enable deployment of Cilium on a cluster with custom helm values, specify the following:

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
              provider: Cilium
              strategy: HelmAddon
              values:
                sourceRef:
                  name: <NAME> #name of ConfigMap present in same namespace
                  kind: <ConfigMap>
```

NOTE: Only ConfigMap kind objects will be allowed to refer helm values from.

ConfigMap Format -

```yaml
apiVersion: v1
data:
  values.yaml: |-
    cni:
      chainingMode: portmap
      exclusive: false
    ipam:
      mode: kubernetes
kind: ConfigMap
metadata:
  name: <CLUSTER_NAME>-cilium-cni-helm-values-template
  namespace: <CLUSTER_NAMESPACE>
```

NOTE: ConfigMap should contain complete helm values for Cilium as same will be applied to Cilium helm chart as it is.

### Default Cilium Specification

Please check the [default Cilium configuration].

## Select Deployment Strategy

To deploy the addon via `ClusterResourceSet` replace the value of `strategy` with `ClusterResourceSet`.

## Calico

Deployment of Calico is opt-in via the  [provider-specific cluster configuration]({{< ref ".." >}}).

### Calico Example

To enable deployment of Calico on a cluster, specify the following values:

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
              provider: Calico
              strategy: HelmAddon
```

### ClusterResourceSet strategy

To deploy the addon via `ClusterResourceSet` replace the value of `strategy` with `ClusterResourceSet`.

When using the `ClusterResourceSet` strategy, the hook creates two `ClusterResourceSets`: one to deploy the Tigera
Operator, and one to deploy Calico via the Tigera `Installation` CRD. The Tigera Operator CRS is shared between all
clusters in the operator, whereas the Calico installation CRS is unique per cluster.

As ClusterResourceSets must exist in the same name as the cluster they apply to, the lifecycle hook copies default
ConfigMaps from the same namespace as the CAPI runtime extensions hook pod is running in. This enables users to
configure defaults specific for their environment rather than compiling the defaults into the binary.

The Helm chart comes with default configurations for the Calico Installation CRS per supported provider, but overriding
is possible. For example. to change Docker provider's Calico configuration, specify following helm argument when
deploying cluster-api-runtime-extensions-nutanix chart:

```shell
--set-file hooks.cni.calico.crsStrategy.defaultInstallationConfigMaps.DockerCluster.configMap.content=<file>
```

[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
[default Cilium configuration]: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/blob/v{{< param "version" >}}/charts/cluster-api-runtime-extensions-nutanix/addons/cni/cilium/values-template.yaml {{< mdl-disable "<!-- markdownlint-disable MD013 MD034 -->" >}}
