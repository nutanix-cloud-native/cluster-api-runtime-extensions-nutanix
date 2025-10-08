+++
title = "Kubernetes cluster registration addon"
icon = "fa-solid fa-object-group"
+++

By leveraging CAPI cluster lifecycle hooks, this handler deploys [Kubernetes cluster registration agent]
on the new cluster at the `AfterControlPlaneInitialized` phase.

The hook uses the [Cluster API Add-on Provider for Helm] to deploy the k8s-registration resources.

## Example

To enable deployment of kubernets registration agent on a cluster, specify the following values:

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
          addons:
            k8sRegistrationAgent: {}
```

[Kubernetes cluster registration agent]: https://github.com/nutanix-core/k8s-agent
[Cluster API Add-on Provider for Helm]: https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm
