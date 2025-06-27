+++
title = "Network Time Protocol (NTP)"
+++

You can configure cluster nodes (control plane, and workers) to update their system clock from a specific set of NTP servers.

Keep in mind that each node's operating system is almost certainly configured with a default set of NTP servers.
In some cases, the node must use different NTP servers. For example, if the node cannot reach the default servers.

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
            ntp
              servers:
                - time1.example.com
                - time2.example.com
```

Applying this configuration will result in the following values being set

for the control plane, on the `KubeadmControlPlaneTemplate`

```yaml
spec:
  kubeadmConfigSpec:
    ntp:
      enabled: true
      servers:
        - time1.example.com
        - time2.example.com
```

and for every worker, on its respective `KubeadmConfigTemplate`

```yaml
spec:
  ntp:
    enabled: true
    servers:
      - time1.example.com
      - time2.example.com
```
