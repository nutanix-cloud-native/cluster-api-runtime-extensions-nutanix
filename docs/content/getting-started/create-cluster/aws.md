+++
title = "AWS"
icon = "fa-brands fa-aws"
weight = 1
+++

Cluster API requires that `ClusterClasses` referenced by a `Cluster` reside in the same namespace as the `Cluster`. To
create the necessary `ClusterClass`, run:

```shell
kubectl apply --server-side \
  -f https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/aws-cluster-class.yaml
```

You can then create your cluster. First, let's list the required variables:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/aws-cluster-cilium-helm-addon.yaml \
  --list-variables
```

Export the required variables and any optional variables that you may want to set:

```shell
export AMI_LOOKUP_BASEOS=<value> \
       AMI_LOOKUP_FORMAT=<value> \
       AMI_LOOKUP_ORG=<value>
```

And create your cluster:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/aws-cluster-cilium-helm-addon.yaml | kubectl apply --server-side -f -
```

To customize your cluster configuration prior to creation, generate the cluster definition to a file and edit it before applying:

```shell
clusterctl generate cluster my-cluster \
  --from https://github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/releases/download/v{{< param "version" >}}/aws-cluster-cilium-helm-addon.yaml >mycluster.yaml

# EDIT mycluster.yaml

kubectl apply --server-side -f mycluster.yaml
```
