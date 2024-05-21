+++
title = "Docker"
icon = "fa-brands fa-docker"
weight = 3
+++

Cluster API requires that `ClusterClasses` referenced by a `Cluster` reside in the same namespace as the `Cluster`. To
create the necessary `ClusterClass`, run:

```shell
kubectl apply --server-side \
  -f https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/docker-cluster-class.yaml
```

You can then create your cluster. First, let's list the required variables:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/docker-cluster-cilium-helm-addon.yaml \
  --list-variables
```

Export the required variables and any optional variables that you may want to set and then create your cluster:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/docker-cluster-cilium-helm-addon.yaml \
  --kubernetes-version={{< param "defaultKubernetesVersion" >}} \
  --worker-machine-count=1 \
  | kubectl apply --server-side -f -
```

To customize your cluster configuration prior to creation, generate the cluster definition to a file and edit it before applying:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/docker-cluster-cilium-helm-addon.yaml \
  --kubernetes-version={{< param "defaultKubernetesVersion" >}} >mycluster.yaml

# EDIT mycluster.yaml

kubectl apply --server-side -f mycluster.yaml
```
