+++
title = "Prism Central Endpoint"
+++

Configure Prism Central Endpoint to create machines on.

## Examples

### Set Prism Central Endpoint

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
            prismCentralEndpoint:
              credentials:
                name: secret-name
              host: x.x.x.x
              insecure: false
              port: 9440
```

Applying this configuration will result in the following value being set:

- control-plane NutanixClusterTemplate:

```yaml
spec:
  template:
    spec:
      prismCentral:
        address: x.x.x.x
        insecure: false
        port: 9440
        credentialRef:
          kind: Secret
          name: secret-name
```
