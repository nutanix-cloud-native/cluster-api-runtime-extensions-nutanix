+++
title = "Service Loadbalancer"
+++

When an application running in a cluster needs to be exposed outside of the cluster, one option is
to use an [external loadbalancer][external-loadbalancer], by creating a Kubernetes Service of the
LoadBalancer type.

The Service Loadbalancer is the component that backs this Kubernetes Service, either by creating
a Virtual IP, creating a machine that runs loadbalancer software, by delegating to APIs, such as
the underlying infrastructure, or a hardware load balancer.

CAREN currently supports the following Service Loadbalancers:

- [MetalLB][metallb]

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
            serviceLoadbalancer:
              provider: MetalLB
```

See [MetalLB documentation][metallb-configuration] for details on configuration.

[external-loadbalancer]: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/
[metallb]: https://metallb.org
[metallb-configuration]: https://metallb.org/configuration/
