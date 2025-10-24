+++
title = "EKS Placement Group Node Feature Discovery"
+++

The EKS placement group NFD (Node Feature Discovery) customization automatically discovers and labels EKS worker nodes with their placement group information, enabling workload scheduling based on placement group characteristics.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## What is Placement Group NFD?

Placement Group NFD automatically discovers the placement group information for each EKS worker node and creates node labels that can be used for workload scheduling. This enables:

- **Workload Affinity**: Schedule pods on nodes within the same placement group for low latency
- **Fault Isolation**: Schedule critical workloads on nodes in different placement groups
- **Resource Optimization**: Use placement group labels for advanced scheduling strategies

## How it Works

The NFD customization:

1. **Deploys a Discovery Script**: Automatically installs a script on each EKS worker node that queries AWS metadata
2. **Queries AWS Metadata**: Uses EC2 instance metadata to discover placement group information
3. **Creates Node Labels**: Generates Kubernetes node labels with placement group details
4. **Updates Continuously**: Refreshes labels as nodes are added or moved

## Generated Node Labels

The NFD customization creates the following node labels:

| Label | Description | Example |
|-------|-------------|---------|
| `feature.node.kubernetes.io/aws-placement-group` | The name of the placement group | `my-eks-worker-pg` |
| `feature.node.kubernetes.io/partition` | The partition number (for partition placement groups) | `0`, `1`, `2` |

## Configuration

The placement group NFD customization is automatically enabled when a placement group is configured for EKS workers. No additional configuration is required.

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

## Usage Examples

### Workload Affinity

Schedule pods on nodes within the same placement group for low latency:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: high-performance-app
spec:
  replicas: 3
  selector:
    matchLabels:
      app: high-performance-app
  template:
    metadata:
      labels:
        app: high-performance-app
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: feature.node.kubernetes.io/aws-placement-group
                operator: In
                values: ["eks-worker-pg"]
      containers:
      - name: app
        image: my-app:latest
```

### Fault Isolation

Distribute critical workloads across different placement groups:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: critical-app
spec:
  replicas: 6
  selector:
    matchLabels:
      app: critical-app
  template:
    metadata:
      labels:
        app: critical-app
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app
                operator: In
                values: ["critical-app"]
            topologyKey: feature.node.kubernetes.io/aws-placement-group
      containers:
      - name: app
        image: critical-app:latest
```

### Partition-Aware Scheduling

For partition placement groups, schedule workloads on specific partitions:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: distributed-database
spec:
  replicas: 3
  selector:
    matchLabels:
      app: distributed-database
  template:
    metadata:
      labels:
        app: distributed-database
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: feature.node.kubernetes.io/partition
                operator: In
                values: ["0", "1", "2"]
      containers:
      - name: database
        image: my-database:latest
```

## Verification

You can verify that the NFD labels are working by checking the node labels:

```bash
# Check all nodes and their placement group labels
kubectl get nodes --show-labels | grep placement-group

# Check specific node labels
kubectl describe node <node-name> | grep placement-group

# Check partition labels
kubectl get nodes --show-labels | grep partition
```

## Troubleshooting

### Check NFD Script Status

Verify that the discovery script is running:

```bash
# Check if the script exists on nodes
kubectl debug node/<node-name> -it --image=busybox -- chroot /host ls -la /etc/kubernetes/node-feature-discovery/source.d/

# Check script execution
kubectl debug node/<node-name> -it --image=busybox -- chroot /host cat /etc/kubernetes/node-feature-discovery/features.d/placementgroup
```

## Integration with Other Features

Placement Group NFD works seamlessly with:

- **Pod Affinity/Anti-Affinity**: Use placement group labels for advanced scheduling
- **Topology Spread Constraints**: Distribute workloads across placement groups

## Security Considerations

- The discovery script queries AWS instance metadata (IMDSv2)
- No additional IAM permissions are required beyond standard EKS node permissions
- Labels are automatically managed and do not require manual intervention
- The script runs with appropriate permissions and security context
