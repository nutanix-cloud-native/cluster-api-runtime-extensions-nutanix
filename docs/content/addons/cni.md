+++
title = "CNI"
icon = "fa-solid fa-network-wired"
+++

When deploying a cluster with CAPI, deployment and configuration of CNI is up to the user. By leveraging CAPI cluster
lifecycle hooks, this handler deploys a requested CNI provider on the new cluster at the `AfterControlPlaneInitialized`
phase.

The hook uses either the [Cluster API Add-on Provider for Helm] or `ClusterResourceSet` to deploy the CNI resources
depending on the selected deployment strategy.

Currently the hook supports [Cilium](#cilium), [Calico](#calico), and [Flow](#flow) (Nutanix only) CNI providers, as
well as the optional [Multus](#multus) meta-CNI addon alongside any primary CNI provider.

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

ConfigMap Format:

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

### Cilium Load Balancer Mode

When kube-proxy is disabled, Cilium replaces it, and forwards traffic for LoadBalancer and NodePort Services using SNAT
mode. Direct Server Return (DSR) mode is not the default, because it is incompatible with some underlay networks. To
use DSR for a specific Service, add the following annotation to the Service:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    service.cilium.io/forwarding-mode: dsr
```

Earlier versions used DSR mode for all Services. Existing clusters change to SNAT mode when their Kubernetes version is
upgraded. If the cluster has NodePort or LoadBalancer Services that are enrolled in Flow Network Security, the
`NutanixFlowNetworkSecurityDSRToSNAT` preflight check blocks the upgrade until every one of these Services has the
annotation.

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

## Flow

Flow CNI is available for Nutanix clusters only. It uses the `HelmAddon` strategy exclusively
(`ClusterResourceSet` is not supported).

Flow CNI and OVN-Kubernetes need more node CPU and memory than Cilium or Calico. The Nutanix
Flow quick-start example uses 4 vCPU / 8Gi machines.

On a Nutanix VPC the guest NIC (`breth0`) is typically 1442 bytes. OVN-Kubernetes
then fails to start when the overlay MTU is larger than that interface, for example
`MTU (1442) of network interface breth0 is too small for specified overlay MTU (1458)`.
The default Flow values set `defaultMTU` and `nutanix-core-flow-ovn-kubernetes.mtu`
to `1350`, which fits that NIC. Clusters that need a different MTU can replace the
values with a ConfigMap, as shown below.

Flow CNI images are hosted on a private Docker Hub registry (`docker.io/nutanix`). If your
environment does not use a mirror or registry credentials configured via `imageRegistries`, you
must provide an image pull secret so the workload cluster can pull the Flow CNI images.

### Flow Example

To enable deployment of Flow CNI on a Nutanix cluster:

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
              provider: Flow
```

### Flow Example With Image Pull Credentials

Create a `kubernetes.io/dockerconfigjson` Secret in the same namespace as the Cluster on the
management cluster:

```yaml
# You can create the Secret manifest manually:
apiVersion: v1
kind: Secret
metadata:
  name: nutanix-docker-hub-credentials
  namespace: <CLUSTER_NAMESPACE>
type: kubernetes.io/dockerconfigjson
data:
  .dockerconfigjson: <BASE64_ENCODED_DOCKER_CONFIG>
```

Or create it directly with kubectl using your Docker Hub credentials:

```shell
kubectl create secret docker-registry nutanix-docker-hub-credentials \
  --docker-username=<USERNAME> \
  --docker-password=<PASSWORD> \
  --namespace=<CLUSTER_NAMESPACE>
```

Then reference it in the cluster configuration:

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
              provider: Flow
              imagePullCredentials:
                secretRef:
                  name: nutanix-docker-hub-credentials
```

The handler copies the Secret to the `flow-cni-system`, `flow-cns-system`, and
`ovn-kubernetes` namespaces on the workload cluster and configures the Flow CNI
Helm chart to use it for pulling images.

### Flow Example With Custom Values

To provide custom Helm values for Flow CNI, create a ConfigMap and reference it:

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
              provider: Flow
              imagePullCredentials:
                secretRef:
                  name: nutanix-docker-hub-credentials
              values:
                sourceRef:
                  name: <NAME> #name of ConfigMap present in same namespace
                  kind: ConfigMap
```

### Default Flow Specification

Please check the [default Flow CNI configuration].

## Multus

Multus is an optional meta-CNI addon that enables attaching multiple network interfaces to pods alongside the primary
CNI (`Cilium`, `Calico`, or `Flow`) across all supported infrastructure providers (AWS, Docker, EKS, and Nutanix).
Multus uses the `HelmAddon` strategy exclusively (`ClusterResourceSet` is not supported, though the primary CNI itself
may use either `HelmAddon` or `ClusterResourceSet`).

Generated quick-start cluster manifests pre-populate `multus: {}` under `addons.cni`. Remove the `multus` field from
the cluster manifest before creating the cluster if you do not want Multus deployed.

### Multus Example

To enable Multus alongside the primary CNI:

```yaml
apiVersion: cluster.x-k8s.io/v1beta2
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
              multus: {}
```

### Multus Example With Custom Values

To customize the Multus Helm release, create a `ConfigMap` in the same namespace as the `Cluster` with a `values.yaml`
key and reference it via `addons.cni.multus.values.sourceRef`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <CLUSTER_NAME>-multus-helm-values
  namespace: <CLUSTER_NAMESPACE>
data:
  values.yaml: |-
    daemonConfig:
      logLevel: verbose
    resources:
      limits:
        memory: 256Mi
---
apiVersion: cluster.x-k8s.io/v1beta2
kind: Cluster
metadata:
  name: <NAME>
  namespace: <CLUSTER_NAMESPACE>
spec:
  topology:
    variables:
      - name: clusterConfig
        value:
          addons:
            cni:
              provider: Cilium
              multus:
                strategy: HelmAddon
                values:
                  sourceRef:
                    kind: ConfigMap
                    name: <CLUSTER_NAME>-multus-helm-values
```

The `values.yaml` content is rendered as a Go template with `{{ .ReadinessSocketPath }}` available. If
`daemonConfig.readinessIndicatorFile` is omitted or empty in custom values, CAREN automatically populates it with the
readiness socket path of the configured primary CNI (`/run/cilium/cilium.sock` for Cilium,
`/var/run/calico/cni-server.sock` for Calico, or `/var/run/ovn-kubernetes/cni/ovn-cni-server.sock` for Flow) so Multus
waits for the primary CNI before advertising readiness.

When supplying custom Cilium Helm values (`addons.cni.values.sourceRef`) alongside Multus, ensure `cni.exclusive: false`
is set in the Cilium values so Cilium does not overwrite the Multus CNI configuration (the default Cilium configuration
already sets `cni.exclusive: false`).

### Existing Clusters and Day-2 Removal

Nutanix and EKS clusters created before `addons.cni.multus` was introduced already have a `multus-<cluster-uuid>`
`HelmChartProxy` without `addons.cni.multus` set in `clusterConfig`. During cluster upgrades, CAREN detects the
existing `HelmChartProxy` and continues reconciling it with the default configuration without modifying `Cluster.spec`.

Removing `addons.cni.multus` from a running cluster stops CAREN from reconciling Multus; to uninstall the release,
delete the `multus-<cluster-uuid>` `HelmChartProxy` in the cluster's namespace on the management cluster, then roll
workload cluster nodes if stale Multus CNI configuration files remain under `/etc/cni/net.d`.

### Default Multus Specification

Please check the [default Multus configuration].

[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm

{{< mdl-disable "<!-- markdownlint-disable MD013 MD034 -->" >}}

[default Cilium configuration]: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/blob/v{{< param "version" >}}/charts/cluster-api-runtime-extensions-nutanix/addons/cni/cilium/values-template.yaml
[default Flow CNI configuration]: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/blob/v{{< param "version" >}}/charts/cluster-api-runtime-extensions-nutanix/addons/cni/nutanix-flow/values-template.yaml
[default Multus configuration]: https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/blob/v{{< param "version" >}}/charts/cluster-api-runtime-extensions-nutanix/addons/cni/multus/values-template.yaml

{{< mdl-disable "<!-- markdownlint-restore -->" >}}
