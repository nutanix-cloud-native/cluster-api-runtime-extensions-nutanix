+++
title = "Extra API Server Certificate SANs"
+++

If the API server can be accessed by alternative DNS addresses then setting additional SANs on the API server
certificate is necessary in order for clients to successfully validate the API server certificate.

This customization will be available when the
[provider-specific cluster configuration patch]({{< ref "..">}}) is included in the `ClusterClass`.

## Example

To add extra SANs to the API server certificate, specify the following configuration:

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
          extraAPIServerCertSANs:
            - a.b.c.example.com
            - d.e.f.example.com
```

Applying this configuration will result in the following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          apiServer:
            certSANs:
              - a.b.c.example.com
              - d.e.f.example.com
