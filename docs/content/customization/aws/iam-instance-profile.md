+++
title = "IAM Instance Profile"
+++

The IAM instance profile customization allows the user to specify the profile to use for control-plane
and worker Machines.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify the IAM instance profile, use the following configuration:

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
              iamInstanceProfile: custom-control-plane.cluster-api-provider-aws.sigs.k8s.io
      - name: workerConfig
        value:
          aws:
            iamInstanceProfile: custom-nodes.cluster-api-provider-aws.sigs.k8s.io
```

Applying this configuration will result in the following value being set:

- control-plane `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          iamInstanceProfile: custom-control-plane.cluster-api-provider-aws.sigs.k8s.io
    ```

- worker `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          iamInstanceProfile: custom-nodes.cluster-api-provider-aws.sigs.k8s.io
    ```
