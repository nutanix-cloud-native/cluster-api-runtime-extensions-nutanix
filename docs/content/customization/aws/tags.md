+++
title = "AWS Additional Tags"
+++

The AWS additional tags customization allows the user to specify custom tags to be applied to AWS resources created by the cluster.
The customization can be applied at the cluster level, control plane level, and worker node level.
This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify additional tags for all AWS resources, use the following configuration:

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
            additionalTags:
              Environment: production
              Team: platform
              CostCenter: "12345"
          controlPlane:
            aws:
              additionalTags:
                NodeType: control-plane
      - name: workerConfig
        value:
          aws:
            additionalTags:
              NodeType: worker
              Workload: general
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
                  aws:
                    additionalTags:
                      NodeType: worker
                      Workload: database
                      Environment: production
```

## Tag Precedence

When tags are specified at multiple levels, the following precedence applies (higher precedence overrides lower):

1. **Worker level tags** and **Control plane level tags** (highest precedence)
1. **Cluster level tags** (lowest precedence)

This means that if the same tag key is specified at multiple levels, the worker and contorl-plane level values will take precedence over the cluster level values.

## Applying this configuration will result in the following values being set

- `AWSCluster`:

  - ```yaml
    spec:
      template:
        spec:
          additionalTags:
            Environment: production
            Team: platform
            CostCenter: "12345"
    ```

- control-plane `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          additionalTags:
            Environment: production
            Team: platform
            CostCenter: "12345"
            NodeType: control-plane
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
