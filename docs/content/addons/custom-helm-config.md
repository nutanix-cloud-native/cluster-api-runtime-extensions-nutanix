+++
title = "Custom Helm Chart Configuration"
icon = "fa-solid fa-eye"
+++

A default helm chart configuration is provided with a ConfigMap `default-helm-addons-config`.
This ConfigMap contains helm chart URL and version for each addon.
A a ConfigMap with customized addon configuration can be created and referenced in the `Cluster` object.

The helm chart configuration for an addon included in the custom ConfigMap will be installed on the cluster.
If configuration for an addon is not included in the the customized addon configuration ConfigMap,
it will be defauled from the `default-helm-addons-config` configmap.

The content of the ConfigMap must follow following specification.

```yaml
  addon-name: |
    ChartName: addon-chart-name
    ChartVersion: 1.0.0
    RepositoryURL: https://my-chart-repository.example.com/charts/
```

Example configmap `custom-helm-addons-config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <CLUSTER_NAME>-helm-addons-config
  namespace: <CLUSTER_NAMESPACE>
  labels:
    clusterctl.cluster.x-k8s.io/move: ""
data:
  nutanix-ccm: |
    ChartName: nutanix-cloud-provider
    ChartVersion: 0.4.2
    RepositoryURL: https://nutanix.github.io/helm/
  nutanix-storage-csi: |
    ChartName: nutanix-csi-storage
    ChartVersion: 3.2.0
    RepositoryURL: https://nutanix.github.io/helm-releases/
```

create the custom configmap in the same namespace as the `Cluster`

```shell
kubectl create -f custom-helm-addons-config.yaml
```

## Example

To install addons using custom helm configuration, specify following values:

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
            helmChartConfig:
              configMapRef:
                name: <NAME>-helm-addons-config
```
