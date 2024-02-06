+++
title = "Global Image Registry Mirror"
+++

Add containerd image registry mirror configuration to all Nodes in the cluster.

When the `globalImageRegistryMirror` variable is set, `files` with configurations for
[Containerd default mirror](https://github.com/containerd/containerd/blob/main/docs/hosts.md).

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To provide an image registry mirror with a CA certificate, specify the following configuration:

If the registry mirror requires a private or self-signed CA certificate,
create a Kubernetes Secret with the `ca.crt` key populated with the CA certificate in PEM format:

```shell
kubectl create secret generic my-mirror-ca-cert \
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
            url: https://example.com
            credentials:
              secretRef:
                name: my-mirror-ca-cert
```

Applying this configuration will result in following new files on the
`KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate` resources:

- `/etc/containerd/certs.d/_default/hosts.toml`
- `/etc/certs/mirror.pem`

To use a public hosted image registry (e.g. ECR) as a registry mirror, specify the following configuration:

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
`KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate` resources:

- `/etc/containerd/certs.d/_default/hosts.toml`
