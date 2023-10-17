+++
title = "Network"
+++

The network customization allows the user to specify existing infrastructure to use for the cluster.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify existing AWS VPC, use the following configuration:

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
            network:
              vpc:
                id: vpc-1234567890
```

To also specify existing AWS Subnets, use the following configuration:

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
            network:
              vpc:
                id: vpc-1234567890
              subnets:
                - id: subnet-1
                - id: subnet-2
                - id: subnet-3
```

Applying this configuration will result in the following value being set:

- `AWSClusterTemplate`:

  - ```yaml
    spec:
      network:
        subnets:
        - id: subnet-1
        - id: subnet-2
        - id: subnet-3
        vpc:
          id: vpc-1234567890
    ```
