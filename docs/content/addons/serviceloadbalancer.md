+++
title = "Service LoadBalancer"
+++

When an application running in a cluster needs to be exposed outside of the cluster, one option is
to use an [external load balancer], by creating a Kubernetes Service of the
`LoadBalancer` type.

The Service Load Balancer is the component that backs this Kubernetes Service, either by creating
a Virtual IP, creating a machine that runs load balancer software, by delegating to APIs, such as
the underlying infrastructure, or a hardware load balancer.

CAREN currently supports the following Service Load Balancers:

- [MetalLB]

## Example

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

See [MetalLB documentation] for details on configuration.

[external load balancer]: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/
[MetalLB]: https://metallb.org
[MetalLB documentation]: https://metallb.org/configuration/
