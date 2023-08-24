---
title: "HTTP proxy for CAPI components"
---

In some network environments it is necessary to use HTTP proxy to successfuly execute HTTP requests.
To configure Kubernetes components (`containerd`, `kubelet`) to use HTTP proxy use the `http-proxy`
external patch that will generate appropriate configuration for control plane and worker nodes.

To enable the http proxy enable the `http-proxy` external patch on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: http-proxy
      external:
        generateExtension: "http-proxy-inject.<external-config-name>"
        discoverVariablesExtension: "http-proxy-vars.<external-config-name>"
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
      name: proxy
      values:
        http: http://example.com
        https: http://example.com
        no:
          - http://no-proxy-1.example.com
          - http://no-proxy-2.example.com
```

Applying this configuration will result in new bootstrap files on the `KubeadmControlPlaneTemplate`
and `KubeadmConfigTemplate`.

This hook is enabled by default, and can be explicitly disabled by omitting the `http-proxy-vars`
and `http-proxy-inject` hook from the `--runtimehooks.enabled-handlers` flag.

If deploying via Helm, then this can be disabled by setting `handlers.http-proxy-vars.enabled=false` and
`handlers.http-proxy-inject.enabled=false`.
