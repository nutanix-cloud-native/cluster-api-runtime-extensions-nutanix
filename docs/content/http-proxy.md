---
title: "HTTP proxy"
---

In some network environments it is necessary to use HTTP proxy to successfuly execute HTTP requests.
To configure Kubernetes components (`containerd`, `kubelet`) to use HTTP proxy use the `httpproxypatch`
external patch that will generate appropriate configuration for control plane and worker nodes.

To enable the meta handler enable the `clusterconfigvars` and `clusterconfigpatch` external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: cluster-config
      external:
        generateExtension: "clusterconfigpatch.capi-runtime-extensions"
        discoverVariablesExtension: "clusterconfigvars.capi-runtime-extensions"
```

On the cluster resource then specify desired HTTP proxy values:

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
          proxy:
            http: http://example.com
            https: https://example.com
            additionalNo:
              - no-proxy-1.example.com
              - no-proxy-2.example.com
```

The `additionalNo` list will be added to default pre-calculated values that apply on k8s networking
`localhost,127.0.0.1,<POD CIDRS>,<SERVICE CIDRS>,kubernetes,kubernetes.default,.svc,.svc.cluster.local`.

Applying this configuration will result in new bootstrap files on the `KubeadmControlPlaneTemplate`
and `KubeadmConfigTemplate`.
