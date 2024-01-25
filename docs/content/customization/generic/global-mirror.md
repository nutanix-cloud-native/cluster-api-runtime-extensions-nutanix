+++
title = "Global Image Registry Mirror"
+++

Add containerd image registry mirror configuration to all Nodes in the cluster.

When the `globalImageRegistryMirror` variable is set, `files` with configurations for [Containerd default mirror](https://github.com/containerd/containerd/blob/main/docs/hosts.md#setup-default-mirror-for-all-registries) will be added.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To provide image registry mirror with CA certificate, specify the following configuration:

If your registry mirror requires a private or self-signed CA certificate,
create a Kubernetes Secret with the `ca.crt` key populated with the CA certificate in PEM format:

```shell
kubectl create secret generic my-mirror-ca-cert-secret \
  --from-file=ca.crt=registry-ca.crt
```

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
          globalImageRegistryMirror:
            url: https://my-mirror.io
            credentials:
              secretRef:
                name: my-mirror-ca-cert-secret
```

Applying this configuration will result in following new files on the
`KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate`

- `/etc/containerd/certs.d/_default/hosts.toml`
- `/etc/certs/mirror.pem`

To use a public hosted image registry (ex. ECR) as mirror, specify the following configuration:

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
          globalImageRegistryMirror:
            url: https://123456789.dkr.ecr.us-east-1.amazonaws.com
```

Applying this configuration will result in following new files on the
`KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate`

- `/etc/containerd/certs.d/_default/hosts.toml`
