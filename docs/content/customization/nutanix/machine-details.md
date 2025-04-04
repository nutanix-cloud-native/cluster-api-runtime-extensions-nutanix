+++
title = "Machine Details"
+++

Configure Machine Details of Control plane and Worker nodes

## Examples

### (Required) Set Machine details for Control Plane and Worker nodes

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

### (Optional) Use Image template

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
                imageLookup:
                  baseOS: "rockylinux-9"
                  format: {{.BaseOS}}-kube-v{{.K8sVersion}}.*
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
      imageLookup:
        baseOS: "rockylinux-9"
        format: {{.BaseOS}}-kube-v{{.K8sVersion}}.*
      memorySize: 4Gi
      providerID: nutanix://vm-uuid
      subnet:
      - name: subnet-name
        type: name
      systemDiskSize: 40Gi
      vcpuSockets: 2
      vcpusPerSocket: 1
```

### (Optional) Set Additional Categories for Control Plane and Worker nodes

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
                additionalCategories:
                - key: example-key
                  value: example-value
      - name: workerConfig
        value:
          nutanix:
            machineDetails:
              additionalCategories:
              - key: example-key
                value: example-value
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
      additionalCategories:
      - key: example-key
        value: example-value
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
      additionalCategories:
      - key: example-key
        value: example-value
```

### (Optional) Set Project for Control Plane and Worker nodes

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
                project:
                  type: name
                  name: project-name
      - name: workerConfig
        value:
          nutanix:
            machineDetails:
              project:
                type: name
                name: project-name
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
      project:
        type: name
        name: project-name
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
      project:
        type: name
        name: project-name
```

### (Optional) Add a GPU to a machine deployment

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
        nutanix:
          machineDetails:
            gpus:
            - type: name
              name: "Ampere 40"
    workers:
      - class: nutanix-quick-start-worker
        metadata:
          annotations:
            cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size: "1"
            cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size: "1"
        name: gpu-0
```

Applying this configuration will result in the following value being set:

- control-plane `NutanixMachineTemplate`:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: NutanixMachineTemplate
metadata:
  name: nutanix-quick-start-gpu-nmt
spec:
  template:
    spec:
      gpus:
      - type: name
        name: "Ampere 40"
```
