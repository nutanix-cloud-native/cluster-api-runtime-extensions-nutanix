+++
title = "Custom image"
+++

The custom image customization allows the user to specify the OCI image to use for control-plane and worker Machines.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To specify the custom image, use the following configuration:

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
            docker:
              customImage: ghcr.io/mesosphere/kind-node:v1.2.3-cp
      - name: workerConfig
        value:
          docker:
            customImage: ghcr.io/mesosphere/kind-node:v1.2.3-worker
```

The configuration above will apply customImage to all workers.
You can further customize individual MachineDeployments by using the `overrides` field with the following configuration:

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
                  docker:
                    customImage: ghcr.io/mesosphere/kind-node:v1.2.3-custom
```

Applying this configuration will result in the following value being set:

- control-plane `DockerMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          customImage: ghcr.io/mesosphere/kind-node:v1.2.3-cp
    ```

- worker `DockerMachineTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          customImage: ghcr.io/mesosphere/kind-node:v1.2.3-worker
    ```
