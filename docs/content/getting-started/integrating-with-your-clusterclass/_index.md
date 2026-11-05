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

The handler names include a version, such as `v7`. Upgrading CAREN never changes what a released version does, so
Clusters using it are not changed by an upgrade. To get new behavior, change both the patch and discovery names to a
newer version; this can roll out Machines. Always use the same version for both.

The required values are shown below per provider for a `cluster.x-k8s.io/v1beta2` `ClusterClass`.

## AWS

```yaml
  patches:
  - external:
      discoverVariablesExtension: awsclusterconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: awsclusterv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: cluster-config
  - external:
      discoverVariablesExtension: awsworkerconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: awsworkerv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: worker-config
```

## Nutanix

```yaml
  patches:
  - external:
      discoverVariablesExtension: nutanixclusterconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: nutanixclusterv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: cluster-config
  - external:
      discoverVariablesExtension: nutanixworkerconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: nutanixworkerv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: worker-config
```

## Docker (for development and testing only)

```yaml
  patches:
  - external:
      discoverVariablesExtension: dockerclusterconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: dockerclusterv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: cluster-config
  - external:
      discoverVariablesExtension: dockerworkerconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: dockerworkerv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: worker-config
```

## Generic (any infrastructure provider)

```yaml
  patches:
  - external:
      discoverVariablesExtension: genericclusterconfigvarsv7-dv.cluster-api-runtime-extensions-nutanix
      generatePatchesExtension: genericclusterv7configpatch-gp.cluster-api-runtime-extensions-nutanix
    name: cluster-config
```

[^1]: Generic runtime hooks only include `clusterConfig` variable as there are no generic worker customizations
    currently available.

[extension config]: https://cluster-api.sigs.k8s.io/tasks/experimental-features/runtime-sdk/implement-extensions.html#extensionconfig
