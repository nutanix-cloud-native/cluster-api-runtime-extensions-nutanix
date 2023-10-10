+++
title = "Instance type"
+++

The instance type customization allows the user to specify the profile to use for control-plane
and worker Machines.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify the instance type, use the following configuration:

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
          controlPlane:
            aws:
              instanceType: m5.xlarge
      - name: workerConfig
        value:
          aws:
            instanceType: m5.2xlarge
```

Applying this configuration will result in the following value being set:

- control-plane `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          instanceType: m5.xlarge
    ```

- worker `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          instanceType: m5.2xlarge
    ```
