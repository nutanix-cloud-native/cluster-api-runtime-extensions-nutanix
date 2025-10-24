+++
title = "EKS Placement Group"
+++

The EKS placement group customization allows the user to specify placement groups for EKS worker nodes to control their placement strategy within AWS.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## What are Placement Groups?

AWS placement groups are logical groupings of instances within a single Availability Zone that influence how instances are placed on underlying hardware. They are useful for:

- **Cluster Placement Groups**: For applications that benefit from low network latency, high network throughput, or both
- **Partition Placement Groups**: For large distributed and replicated workloads, such as HDFS, HBase, and Cassandra
- **Spread Placement Groups**: For applications that have a small number of critical instances that should be kept separate

## Configuration

The placement group configuration supports the following field:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | The name of the placement group (1-255 characters) |

## Examples

### EKS Worker Placement Groups

To specify placement groups for EKS worker nodes:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: workerConfig
        value:
          eks:
            placementGroup:
              name: "eks-worker-pg"
```

### Multiple Node Groups with Different Placement Groups

You can configure different placement groups for different node groups:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: workerConfig
        value:
          eks:
            placementGroup:
              name: "general-worker-pg"
    workers:
      machineDeployments:
        - class: high-performance-worker
          name: md-0
          variables:
            overrides:
              - name: workerConfig
                value:
                  eks:
                    placementGroup:
                      name: "high-performance-pg"
        - class: general-worker
          name: md-1
          variables:
            overrides:
              - name: workerConfig
                value:
                  eks:
                    placementGroup:
                      name: "general-worker-pg"
```

## Resulting EKS Configuration

Applying the placement group configuration will result in the following value being set in the EKS worker node configuration:

- worker `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          placementGroupName: worker-pg
    ```

## Best Practices

1. **Placement Group Types**: Choose the appropriate placement group type based on your workload:
   - **Cluster**: For applications requiring low latency and high throughput
   - **Partition**: For large distributed workloads that need fault isolation
   - **Spread**: For critical instances that need maximum availability

2. **Naming Convention**: Use descriptive names that indicate the purpose and type of the placement group

3. **Availability Zone**: Placement groups are constrained to a single Availability Zone, so plan your cluster topology accordingly

4. **Instance Types**: Some instance types have restrictions on placement groups (e.g., some bare metal instances)

5. **Capacity Planning**: Consider the placement group capacity limits when designing your cluster

6. **EKS Node Groups**: Consider using different placement groups for different node groups based on workload requirements

## Important Notes

- Placement groups must be created in AWS before they can be referenced
- Placement groups are constrained to a single Availability Zone
- You cannot move an existing instance into a placement group
- Some instance types cannot be launched in placement groups
- Placement groups have capacity limits that vary by type and instance family
- EKS managed node groups support placement groups for enhanced networking performance
