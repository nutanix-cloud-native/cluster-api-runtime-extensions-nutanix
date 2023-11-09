+++
title = "AWS Additional Security Group Spec"
+++

The AWS additional security group customization allows the user to specify security groups to the created machines.
The customization can be applied to both control plane and nodepool machines.
This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify addiitonal security groups for all control plane and nodepools, use the following configuration:

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
              additionalSecurityGroups:
              - id: "sg-0fcfece738d3211b8"
      - name: workerConfig
        value:
          aws:
            additionalSecurityGroups:
            - id: "sg-0fcfece738d3211b8"
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
                    additionalSecurityGroups:
                    - id: "sg-0fcfece738d3211b8"
```

Applying this configuration will result in the following value being set:

- control-plane `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          additionalSecurityGroups:
          - id: sg-0fcfece738d3211b8
    ```

- worker `AWSMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          additionalSecurityGroups:
          - id: sg-0fcfece738d3211b8
    ```
