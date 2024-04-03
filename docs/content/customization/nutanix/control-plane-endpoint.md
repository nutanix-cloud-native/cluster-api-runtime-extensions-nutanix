+++
title = "Control Plane Endpoint"
+++

Configure Control Plane Endpoint. Defines the host IP and port of the CAPX Kubernetes cluster.

## Examples

### Set Control Plane Endpoint

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
          nutanix:
            controlPlaneEndpoint:
              host: x.x.x.x
              port: 6443
```

Applying this configuration will result in the following value being set:

- `NutanixCluster`:

```yaml
spec:
  template:
    spec:
      controlPlaneEndpoint:
        host: x.x.x.x
        port: 6443
```
