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

## Examples

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

[external load balancer]: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/
[MetalLB]: https://metallb.org
[MetalLB documentation]: https://metallb.org/configuration/
