+++
title = "Via clusterctl"
icon = "fa-solid fa-circle-nodes"
weight = 1
+++

Add the following to your `clusterctl.yaml` file, which is normally found at
`${XDG_CONFIG_HOME}/cluster-api/clusterctl.yaml` (or `${HOME}/cluster-api/clusterctl.yaml`). See [clusterctl
configuration file] for more details. If the `providers` section already exists, add the entry and omit the `providers`
key from this block below:

```yaml
providers:
  - name: "caren"
    url: "https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/latest/runtime-extension-components.yaml"
    type: "RuntimeExtensionProvider"
```

Now we can deploy CAREN and other necessary providers (update infrastructure providers for your needs), leaving all
configuration values blank as we will specify these when creating clusters:

```shell
env CLUSTER_TOPOLOGY=true \
    EXP_RUNTIME_SDK=true  \
    EXP_CLUSTER_RESOURCE_SET=true  \
    NUTANIX_ENDPOINT= NUTANIX_PASSWORD= NUTANIX_USER=  \
    AWS_B64ENCODED_CREDENTIALS=  \
    clusterctl init \
      --infrastructure docker,nutanix:v1.4.0-alpha.2,aws \
      --addon helm \
      --runtime-extension caren \
      --wait-providers
```

[clusterctl configuration file]: https://cluster-api.sigs.k8s.io/clusterctl/configuration.html?highlight=clusterctl%20config#variables
