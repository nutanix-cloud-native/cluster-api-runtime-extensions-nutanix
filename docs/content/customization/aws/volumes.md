+++
title = "AWS Volumes Configuration"
+++

The AWS volumes customization allows the user to specify configuration for both root and non-root storage volumes for AWS machines.
The volumes customization can be applied to both control plane and worker machines.
This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Configuration Options

The volumes configuration supports two types of volumes:

- **Root Volume**: The primary storage volume for the instance (typically `/dev/sda1`)
- **Non-Root Volumes**: Additional storage volumes that can be attached to the instance

### Volume Configuration Fields

Each volume can be configured with the following fields:

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `deviceName` | string | No | Device name for the volume (e.g., `/dev/sda1`, `/dev/sdf`) | - |
| `size` | int64 | No | Size in GiB (minimum 8) | Based on AMI, usually 20GiB |
| `type` | string | No | EBS volume type (`gp2`, `gp3`, `io1`, `io2`) | - |
| `iops` | int64 | No | IOPS for provisioned volumes (io1, io2, gp3) | - |
| `throughput` | int64 | No | Throughput in MiB/s (gp3 only) | - |
| `encrypted` | bool | No | Whether the volume should be encrypted | false |
| `encryptionKey` | string | No | KMS key ID or ARN for encryption | AWS default key |

### Supported Volume Types

- **gp2**: General Purpose SSD (up to 16,000 IOPS)
- **gp3**: General Purpose SSD with configurable IOPS and throughput
- **io1**: Provisioned IOPS SSD (up to 64,000 IOPS)
- **io2**: Provisioned IOPS SSD with higher durability (up to 64,000 IOPS)

## Examples

### Root Volume Only

To specify only a root volume configuration:

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
              volumes:
                root:
                  deviceName: "/dev/sda1"
                  size: 100
                  type: "gp3"
                  iops: 3000
                  throughput: 125
                  encrypted: true
                  encryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012"
      - name: workerConfig
        value:
          aws:
            volumes:
              root:
                size: 200
                type: "gp3"
                iops: 4000
                throughput: 250
                encrypted: true
```

### Non-Root Volumes Only

To specify only additional non-root volumes:

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
              volumes:
                nonroot:
                  - deviceName: "/dev/sdf"
                    size: 500
                    type: "gp3"
                    iops: 4000
                    throughput: 250
                    encrypted: true
                  - deviceName: "/dev/sdg"
                    size: 1000
                    type: "gp2"
                    encrypted: false
      - name: workerConfig
        value:
          aws:
            volumes:
              nonroot:
                - deviceName: "/dev/sdf"
                  size: 200
                  type: "io1"
                  iops: 10000
                  encrypted: true
```

### Both Root and Non-Root Volumes

To specify both root and non-root volumes:

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
              volumes:
                root:
                  size: 100
                  type: "gp3"
                  iops: 3000
                  throughput: 125
                  encrypted: true
                nonroot:
                  - deviceName: "/dev/sdf"
                    size: 500
                    type: "gp3"
                    iops: 4000
                    throughput: 250
                    encrypted: true
                  - deviceName: "/dev/sdg"
                    size: 1000
                    type: "gp2"
                    encrypted: false
      - name: workerConfig
        value:
          aws:
            volumes:
              root:
                size: 200
                type: "gp3"
                iops: 4000
                throughput: 250
                encrypted: true
              nonroot:
                - deviceName: "/dev/sdf"
                  size: 100
                  type: "io1"
                  iops: 10000
                  encrypted: true
```

### MachineDeployment Overrides

You can customize individual MachineDeployments by using the overrides field:

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
                    volumes:
                      root:
                        size: 500
                        type: "gp3"
                        iops: 10000
                        throughput: 500
                        encrypted: true
                      nonroot:
                        - deviceName: "/dev/sdf"
                          size: 1000
                          type: "io2"
                          iops: 20000
                          encrypted: true
```

## Resulting CAPA Configuration

Applying the volumes configuration will result in the following values being set in the `AWSMachineTemplate`:

### Root Volume Configuration

When a root volume is specified, it will be set in the `rootVolume` field:

```yaml
spec:
  template:
    spec:
      rootVolume:
        deviceName: "/dev/sda1"
        size: 100
        type: "gp3"
        iops: 3000
        throughput: 125
        encrypted: true
        encryptionKey: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012"
```

### Non-Root Volumes Configuration

When non-root volumes are specified, they will be set in the `nonRootVolumes` field:

```yaml
spec:
  template:
    spec:
      nonRootVolumes:
        - deviceName: "/dev/sdf"
          size: 500
          type: "gp3"
          iops: 4000
          throughput: 250
          encrypted: true
        - deviceName: "/dev/sdg"
          size: 1000
          type: "gp2"
          encrypted: false
```

## EKS Configuration

For EKS clusters, the volumes configuration follows the same structure but is specified under the EKS worker configuration:

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
            volumes:
              root:
                size: 200
                type: "gp3"
                iops: 4000
                throughput: 250
                encrypted: true
              nonroot:
                - deviceName: "/dev/sdf"
                  size: 500
                  type: "gp3"
                  iops: 4000
                  throughput: 250
                  encrypted: true
```

## Best Practices

1. **Root Volume**: Always specify a root volume for consistent boot disk configuration
2. **Encryption**: Enable encryption for sensitive workloads using either AWS default keys or customer-managed KMS keys
3. **IOPS and Throughput**: Use gp3 volumes for better price/performance ratio with configurable IOPS and throughput
4. **Device Names**: Use standard device naming conventions (`/dev/sda1` for root, `/dev/sdf` onwards for additional volumes)
5. **Size Planning**: Consider future growth when sizing volumes, as resizing EBS volumes requires downtime
6. **Volume Types**: Choose appropriate volume types based on workload requirements:
   - **gp2/gp3**: General purpose workloads
   - **io1/io2**: High-performance database workloads requiring consistent IOPS
