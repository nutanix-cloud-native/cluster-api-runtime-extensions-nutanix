+++
title = "Nutanix"
icon = "fa-solid fa-server"
weight = 2
+++


Cluster API requires that `ClusterClasses` referenced by a `Cluster` reside in the same namespace as the `Cluster`. To
create the necessary `ClusterClass`, run:

```shell
kubectl apply --server-side \
  -f https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/nutanix-cluster-class.yaml
```

You can then create your cluster. First, let's list the required variables:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/nutanix-cluster-cilium-helm-addon.yaml \
  --list-variables
```

Export the required variables and any optional variables that you may want to set:

```shell
export CONTROL_PLANE_ENDPOINT_IP=<value> \
       DOCKER_HUB_PASSWORD=<value> \
       DOCKER_HUB_USERNAME=<value> \
       NUTANIX_ENDPOINT=<value> \
       NUTANIX_INSECURE=<value> \
       NUTANIX_MACHINE_TEMPLATE_IMAGE_NAME=<value> \
       NUTANIX_PASSWORD=<value> \
       NUTANIX_PORT=<value> \
       NUTANIX_PRISM_ELEMENT_CLUSTER_NAME=<value> \
       NUTANIX_STORAGE_CONTAINER_NAME=<value> \
       NUTANIX_SUBNET_NAME=<value> \
       NUTANIX_USER
```

And create your cluster:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/nutanix-cluster-cilium-helm-addon.yaml | kubectl apply --server-side -f -
```

To customize your cluster configuration prior to creation, generate the cluster definition to a file and edit it before applying:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/nutanix-cluster-cilium-helm-addon.yaml >mycluster.yaml

# EDIT mycluster.yaml

kubectl apply --server-side -f mycluster.yaml
```
