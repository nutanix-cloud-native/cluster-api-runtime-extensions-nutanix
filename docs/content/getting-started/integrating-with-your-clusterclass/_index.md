+++
title = "Integrating with your ClusterClass"
icon = "fa-solid fa-seedling"
weight = 3
+++

CAREN provides an [extension config] for the provided runtime hooks for the various supported providers that you can
integrate with your own `ClusterClass` to add to your own customizations. In this way you can take advantage of what
CAREN provides instead of having to write your own.

To integrate with your `ClusterClasses`, you will need to add the appropriate external patch references to your
`ClusterClass` depending on your infrastructure provider under `spec.patches`. Once you have done this, you will be able
to specify the variables `clusterConfig` and `workerConfig`[^1] regardless of infrastructure provider, although remember
that each variable will have provider-specific fields as part of the schema.

The required values are shown below per provider.

## AWS

```yaml
  patches:
  - external:
      discoverVariablesExtension: awsclusterconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: awsclusterv3configpatch.cluster-api-runtime-extensions-nutanix
    name: cluster-config
  - external:
      discoverVariablesExtension: awsworkerconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: awsworkerv3configpatch.cluster-api-runtime-extensions-nutanix
    name: worker-config
```

## Nutanix

```yaml
  patches:
  - external:
      discoverVariablesExtension: nutanixclusterconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: nutanixclusterv3configpatch.cluster-api-runtime-extensions-nutanix
    name: cluster-config
  - external:
      discoverVariablesExtension: nutanixworkerconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: nutanixworkerv3configpatch.cluster-api-runtime-extensions-nutanix
    name: worker-config
```

## Docker (for development and testing only)

```yaml
  patches:
  - external:
      discoverVariablesExtension: dockerclusterconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: dockerclusterv3configpatch.cluster-api-runtime-extensions-nutanix
    name: cluster-config
  - external:
      discoverVariablesExtension: dockerworkerconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: dockerworkerv3configpatch.cluster-api-runtime-extensions-nutanix
    name: worker-config
```

## Generic (any infrastructure provider)

```yaml
  patches:
  - external:
      discoverVariablesExtension: genericclusterconfigvars.cluster-api-runtime-extensions-nutanix
      generateExtension: genericclusterv3configpatch.cluster-api-runtime-extensions-nutanix
    name: cluster-config
```

[^1]: Generic runtime hooks only include `clusterConfig` variable as there are no generic worker customizations
    currently available.

[extension config]: https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/implement-extensions.html#extensionconfig
