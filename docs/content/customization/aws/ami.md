+++
title = "AWS AMI ID and Format spec"
+++

The AWS AMI customization allows the user to specify the AMI or AMI Lookup arguments for a AWS machine.
The AMI customization can be applied to both control plane and nodepool machines.
This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify the AMI ID or format for all control plane and nodepools, use the following configuration:

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
              ami:
                # Specify one of id or lookup.
                id: "ami-controlplane"
                # lookup:
                #   format: "my-cp-ami-{{.BaseOS}}-?{{.K8sVersion}}-*"
                #   org: "123456789"
                #   baseOS: "ubuntu-20.04"
      - name: workerConfig
        value:
          aws:
            ami:
              # Specify one of id or lookup.
              id: "ami-allWorkers"
              # lookup:
              #   format: "my-default-workers-ami-{{.BaseOS}}-?{{.K8sVersion}}-*"
              #   org: "123456789"
              #   baseOS: "ubuntu-20.04"
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
                   ami:
                    # Specify one of id or lookup.
                    id: "ami-customWorker"
                    # lookup:
                    #   format: "gpu-workers-ami-{{.BaseOS}}-?{{.K8sVersion}}-*"
                    #   org: "123456789"
                    #   baseOS: "ubuntu-20.04"
```

Applying this configuration will result in the following value being set:

- control-plane `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          ami: ami-controlplane
          # lookupFormat: "my-default-workers-ami-{{.BaseOS}}-?{{.K8sVersion}}-*"
          # lookupOrg: "123456789"
          # lookupBaseOS: "ubuntu-20.04"
    ```

- worker `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          ami: ami-customWorker
          # lookupFormat: "gpu-workers-ami-{{.BaseOS}}-?{{.K8sVersion}}-*"
          # lookupOrg: "123456789"
          # lookupBaseOS: "ubuntu-20.04"
    ```
