+++
title = "Service LoadBalancer"
icon = "fa-solid fa-arrows-turn-to-dots"
+++

When an application running in a cluster needs to be exposed outside of the cluster, one option is
to use an [external load balancer], by creating a Kubernetes Service of the
`LoadBalancer` type.

The Service Load Balancer is the component that backs this Kubernetes Service, either by creating
a Virtual IP, creating a machine that runs load balancer software, by delegating to APIs, such as
the underlying infrastructure, or a hardware load balancer.

The Service Load Balancer can choose the Virtual IP from a pre-defined address range. You can use
CAREN to configure one or more IPv4 ranges. For additional options, configure the Service Load
Balancer yourself after it is deployed.

CAREN currently supports the following Service Load Balancers:

- [MetalLB]
- [Cilium] (reuses the Cilium CNI installation)

## Choosing a provider

| Concern          | MetalLB                       | Cilium                                                |
| ---------------- | ----------------------------- | ----------------------------------------------------- |
| Extra dataplane  | Yes (runs in `metallb-system`) | No — reuses the Cilium CNI agent                      |
| Prerequisites    | None                          | Cilium CNI addon and `kubeProxy.mode=disabled`        |
| L2 announcements | Enabled by default            | Enabled (uses Cilium L2 announcement policies)        |
| BGP              | Supported via MetalLB config  | Not exposed through CAREN in this release             |

If the cluster already uses Cilium as its CNI, the Cilium provider avoids deploying a second
dataplane and shares configuration with the existing Cilium install. Otherwise, MetalLB remains
the default choice.

## Examples

### MetalLB

To enable deployment of MetalLB on a cluster, specify the following values:

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
            serviceLoadBalancer:
              provider: MetalLB
```

To enable MetalLB, and configure two address IPv4 ranges, specify the following values:

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
            serviceLoadBalancer:
              provider: MetalLB
              configuration:
                addressRanges:
                - start: 10.100.1.1
                  end: 10.100.1.20
                - start: 10.100.1.51
                  end: 10.100.1.70
```

See [MetalLB documentation] for more configuration details.

### Cilium

The Cilium provider uses the existing Cilium CNI installation to allocate and announce load
balancer IPs. It requires two prerequisites:

1. `addons.cni.provider` is set to `Cilium`.
2. `kubeProxy.mode` is set to `disabled` (Cilium's kube-proxy replacement handles service
    traffic).

Admission will reject the cluster if either prerequisite is missing. Switching providers on an
existing cluster is also rejected; recreate the cluster to change providers.

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
          kubeProxy:
            mode: disabled
          addons:
            cni:
              provider: Cilium
            serviceLoadBalancer:
              provider: Cilium
              configuration:
                addressRanges:
                - start: 10.100.1.1
                  end: 10.100.1.20
```

Under the hood CAREN installs a `CiliumLoadBalancerIPPool` that allocates IPs from the configured
ranges to Services of type `LoadBalancer`, and a `CiliumL2AnnouncementPolicy` that announces the
allocated IPs on the local network via ARP/NDP from every node. See the [Cilium load balancer]
and [Cilium L2 announcement] documentation for more configuration details.

[external load balancer]: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/
[MetalLB]: https://metallb.org
[MetalLB documentation]: https://metallb.org/configuration/
[Cilium]: https://cilium.io
[Cilium load balancer]: https://docs.cilium.io/en/stable/network/lb-ipam/
[Cilium L2 announcement]: https://docs.cilium.io/en/stable/network/l2-announcements/
