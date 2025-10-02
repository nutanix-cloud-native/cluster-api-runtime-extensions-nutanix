+++
title = "EKS Additional Tags"
+++

The EKS additional tags customization allows the user to specify custom tags to be applied to AWS resources created by the EKS cluster.
The customization can be applied at the cluster level and worker node level.
This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify additional tags for EKS resources, use the following configuration:

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
          eks:
            additionalTags:
              Environment: production
              Team: platform
              CostCenter: "12345"
```

We can further customize individual MachineDeployments by using the overrides field with the following configuration:

```yaml
spec:
  topology:
    # ...
    workers:
      machineDeployments:
        - class: default-worker
          name: md-0
          variables:
            overrides:
              - name: workerConfig
                value:
                  eks:
                    additionalTags:
                      NodeType: worker
                      Workload: database
                      Environment: production
```

## Tag Precedence

When tags are specified at multiple levels, the following precedence applies (higher precedence overrides lower):

1. **Worker level tags** (highest precedence)
2. **Cluster level tags** (lowest precedence)

This means that if the same tag key is specified at multiple levels, the worker level values will take precedence over the cluster level values.

## Applying this configuration will result in the following values being set

- `AWSManagedControlPlane`:

  - ```yaml
    spec:
      template:
        spec:
          additionalTags:
            Environment: production
            Team: platform
            CostCenter: "12345"
    ```

- worker `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          additionalTags:
            Environment: production
            Team: platform
            CostCenter: "12345"
            NodeType: worker
            Workload: general
    ```
