+++
title = "Failure Domains"
+++

Configure Failure Domains. Defines the Prism Element Cluster and subnets to use for creating Control Plane or Worker
node VMs of Kubernetes Cluster.

## Examples

### Configure one or more Failure Domains

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
          nutanix:
            failureDomains:
            - cluster:
                name: pe-cluster-name-1
                type: name
              controlPlane: true
              name: failure-domain-name-1
              subnets:
              - name: subnet-name-1
                type: name
            - cluster:
                name: pe-cluster-name-2
                type: name
              controlPlane: true
              name: failure-domain-name-2
              subnets:
              - name: subnet-name-2
                type: name

```

Applying this configuration will result in the following value being set:

- `NutanixCluster`:

```yaml
spec:
  template:
    spec:
      failureDomains:
        - cluster:
            name: pe-cluster-name-1
            type: name
          controlPlane: true
          name: failure-domain-name-1
          subnets:
          - name: subnet-name-1
            type: name
        - cluster:
            name: pe-cluster-name-2
            type: name
          controlPlane: true
          name: failure-domain-name-2
          subnets:
          - name: subnet-name-2
            type: name
```

Note:

- Configuring Failure Domains is optional and if not configured then respective NutanixMachineTemplate's cluster and
subnets will be used to create respective control plane and worker nodes

- Only one Failure Domain can be used per Machine Deployment. Worker nodes will be created on respective Prism Element
cluster and subnet of the respective failure domain.

- Control plane nodes will be created on every failure domain's cluster which has ControlPlane boolean set to true.

Following is the way to set failure Domain to each Machine Deployment

- `NutanixCluster`:

```yaml
    workers:
      machineDeployments:
      - class: default-worker
        name: md-0
        failureDomain: failure-domain-name-1
      - class: default-worker
        name: md-1
        failureDomain: failure-domain-name-2
```
