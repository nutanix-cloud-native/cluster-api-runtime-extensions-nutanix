+++
title = "Region"
+++

The region customization allows the user to specify the region to deploy a cluster into.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify the AWS region to deploy into, use the following configuration:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: clusterConfig
        values:
          region: us-west-2
```

Applying this configuration will result in the following value being set:

- `AWSClusterTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          region: us-west-2
    ```
