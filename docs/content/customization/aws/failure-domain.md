+++
title = "AWS Failure Domain"
+++

The AWS failure domain customization allows the user to specify the AWS availability zone (failure domain) for worker node deployments.
This customization can be applied to individual MachineDeployments to distribute worker nodes across different availability zones for high availability.
This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify a failure domain for worker nodes, use the following configuration:

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
          aws:
            failureDomain: us-west-2a
```

You can customize individual MachineDeployments by using the overrides field to deploy workers across multiple availability zones:

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
                    failureDomain: us-west-2a
        - class: default-worker
          name: md-1
          variables:
            overrides:
              - name: workerConfig
                value:
                  aws:
                    failureDomain: us-west-2b
        - class: default-worker
          name: md-2
          variables:
            overrides:
              - name: workerConfig
                value:
                  aws:
                    failureDomain: us-west-2c
```

## Resulting CAPA Configuration

Applying this configuration will result in the following value being set:

- worker `MachineDeployment`:

  - ```yaml
    spec:
      template:
        spec:
          failureDomain: us-west-2a
    ```
