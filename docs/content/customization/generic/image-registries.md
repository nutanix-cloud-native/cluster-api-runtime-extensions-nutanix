+++
title = "Image registries"
+++

Add image registry configuration to all Nodes in the cluster.

When the `credentials` variable is set, `files` and `preKubeadmnCommands` with configurations for
[Kubelet image credential provider](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/)
and [dynamic credential provider](https://github.com/mesosphere/dynamic-credential-provider) will be added.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

If your registry requires static credentials, create a Kubernetes Secret with keys for `username` and `password`:

```shell
kubectl create secret generic my-registry-credentials \
  --from-literal username=${REGISTRY_USERNAME} --from-literal password=${REGISTRY_PASSWORD}
```

To add image registry credentials, specify the following configuration:

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
          imageRegistries:
            - url: https://my-registry.io
              credentials:
                secretRef:
                  name: my-registry-credentials
```

Applying this configuration will result in new files and preKubeadmCommands
on the `KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate`.

To use a image registry as mirror with CA certificate, specify the following configuration:

If your registry mirror requires self signed CA certifate, create a Kubernetes Secret with keys for `ca.crt`:

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
          imageRegistries:
            - url: https://my-registry.io
              credentials:
                secretRef:
                  name: my-registry-credentials
              mirror:
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
          imageRegistries:
            - url: https://123456789.dkr.ecr.us-east-1.amazonaws.com
              credentials:
                secretRef:
                  name: my-registry-credentials
              mirror: {}
```

Applying this configuration will result in following new files on the
`KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate`

- `/etc/containerd/certs.d/_default/hosts.toml`
