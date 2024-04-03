+++
title = "Machine Details"
+++

Configure Machine Details of Control plane and Worker nodes

## Examples

### Set Machine details of Control Plane and Worker nodes

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
            nutanix:
              machineDetails:
                bootType: legacy
                cluster:
                  name: pe-cluster-name
                  type: name
                image:
                  name: os-image-name
                  type: name
                memorySize: 4Gi
                subnets:
                - name: subnet-name
                  type: name
                systemDiskSize: 40Gi
                vcpuSockets: 2
                vcpusPerSocket: 1
      - name: workerConfig
        value:
          nutanix:
            machineDetails:
              bootType: legacy
              cluster:
                name: pe-cluster-name
                type: name
              image:
                name: os-image-name
                type: name
              memorySize: 4Gi
              subnets:
              - name: subnet-name
                type: name
              systemDiskSize: 40Gi
              vcpuSockets: 2
              vcpusPerSocket: 1
```

Applying this configuration will result in the following value being set:

- control-plane `NutanixMachineTemplate`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: NutanixMachineTemplate
metadata:
  name: nutanix-quick-start-cp-nmt
spec:
  template:
    spec:
      bootType: legacy
      cluster:
        name: pe-cluster-name
        type: name
      image:
        name: os-image-name
        type: name
      memorySize: 4Gi
      providerID: nutanix://vm-uuid
      subnet:
      - name: subnet-name
        type: name
      systemDiskSize: 40Gi
      vcpuSockets: 2
      vcpusPerSocket: 1
```

- worker `NutanixMachineTemplate`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: NutanixMachineTemplate
metadata:
  name: nutanix-quick-start-md-nmt
spec:
  template:
    spec:
      bootType: legacy
      cluster:
        name: pe-cluster-name
        type: name
      image:
        name: os-image-name
        type: name
      memorySize: 4Gi
      providerID: nutanix://vm-uuid
      subnet:
      - name: subnet-name
        type: name
      systemDiskSize: 40Gi
      vcpuSockets: 2
      vcpusPerSocket: 1
```
