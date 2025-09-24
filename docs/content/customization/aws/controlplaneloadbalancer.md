+++
title = "Control Plane Load Balancer"
+++

The control-plane load balancer customization allows the user
to modify the load balancer configuration for the control-plane's API server.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To use an internal ELB scheme, use the following configuration:

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
          aws:
            controlPlaneLoadBalancer:
              scheme: internal
```

Applying this configuration will result in the following value being set:

- `AWSCluster`:

  - ```yaml
    spec:
      controlPlaneLoadBalancer:
        scheme: internal
    ```
