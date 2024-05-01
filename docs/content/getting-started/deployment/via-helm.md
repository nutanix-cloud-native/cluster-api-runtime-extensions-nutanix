+++
title = "Via Helm"
icon = "fa-solid fa-helmet-safety"
weight = 2
+++

When installing CAREN via Helm, we need to deploy Cluster API core providers and any other required infrastructure
providers to our management cluster via `clusterctl`:

```shell
env CLUSTER_TOPOLOGY=true \
    EXP_RUNTIME_SDK=true  \
    EXP_CLUSTER_RESOURCE_SET=true  \
    NUTANIX_ENDPOINT= NUTANIX_PASSWORD= NUTANIX_USER=  \
    AWS_B64ENCODED_CREDENTIALS=  \
    clusterctl init \
      --infrastructure docker,nutanix:v1.4.0-alpha.2,aws \
      --addon helm \
      --wait-providers
```

We can then deploy CAREN via Helm by adding the Helm repo and installing in the usual way via Helm:
Add the CAREN Helm repo:

```shell
helm repo add caren https://nutanix-cloud-native.github.io/cluster-api-runtime-extensions-nutanix/helm
helm repo update caren
helm upgrade --install caren caren/cluster-api-runtime-extensions-nutanix \
  --namespace cluster-api-runtime-extensions-nutanix-system \
  --create-namespace \
  --wait \
  --wait-for-jobs
```
