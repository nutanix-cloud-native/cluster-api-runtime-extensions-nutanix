+++
title = "Image registry credentials"
+++

In some network environments it is necessary to use HTTP proxy to successfuly execute HTTP requests.
To configure Kubernetes components (`containerd`, `kubelet`) to use HTTP proxy use the `httpproxypatch`
external patch that will generate appropriate configuration for control plane and worker nodes.

Add image registry credentials to all Nodes in the cluster.
When this handle is enabled, the handler will add `files` and `preKubeadmnCommands` with configurations for
[Kubelet image credential provider](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/)
and [dynamic credential provider](https://github.com/mesosphere/dynamic-credential-provider).

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

If your registry requires static credentials, create a Kubernetes Secret with keys for `username` and `password`:

```shell
kubectl create secret generic my-registry-credentials \
  --from-literal username=${REGISTRY_USERNAME} password=${REGISTRY_PASSWORD}
```

On the cluster resource then specify desired image registry credentials:

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
          imageRegistryCredentials:
            url: https://my-registry.io
            secretRef:
              name: my-registry-credentials
```

Applying this configuration will result in new files and preKubeadmCommands
on the `KubeadmControlPlaneTemplate` and `KubeadmConfigTemplate`.
