+++
title = "Cluster customizations"
weight = 1
+++

The cluster configuration handlers wrap all the other mutation handlers in a convenient single patch for inclusion in
your ClusterClasses, allowing for a single configuration variable with nested values. This provides the most flexibility
with the least configuration.

To enable the handler, add the provider-specific `clusterconfigvars` and `clusterconfigpatch` external patches on
`ClusterClass`. This will enable all of the [generic cluster customizations]({{< ref "generic" >}}), along with the
relevant provider-specific variables.

Regardless of provider, a single variable called `clusterConfig` will be available for use on the `ClusterClass`. The
schema (and therefore the configuration options) will be customized for each provider. To use the exposed configuration
options, specify the desired values on the `Cluster` resource:

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
          kubernetesImageRepository: "my-registry.io/my-org/my-repo"
          etcd:
            image:
              repository: my-registry.io/my-org/my-repo
              tag: "v3.5.99_custom.0"
          extraAPIServerCertSANs:
            - a.b.c.example.com
            - d.e.f.example.com
          proxy:
            http: http://example.com
            https: https://example.com
            additionalNo:
              - no-proxy-1.example.com
              - no-proxy-2.example.com
          imageRegistryCredentials:
            url: https://my-registry.io
            secret: my-registry-credentials
          cni:
            provider: calico
```

## AWS

See [AWS customizations]({{< ref "aws" >}}) for the AWS specific customizations.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: cluster-config
      external:
        generateExtension: "awsclusterconfigpatch.capi-runtime-extensions"
        discoverVariablesExtension: "awsclusterconfigvars.capi-runtime-extensions"
```

## Docker

See [generic customizations]({{< ref "generic" >}}) for the Docker specific customizations.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: cluster-config
      external:
        generateExtension: "dockerclusterconfigpatch.capi-runtime-extensions"
        discoverVariablesExtension: "dockerclusterconfigvars.capi-runtime-extensions"
```
