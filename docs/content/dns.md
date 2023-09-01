---
title: "DNS"
---

To enable configuration of DNS components enable the `dnsvars` and `dnspatch` external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: dns
      external:
        generateExtension: "dnspatch.<external-config-name>"
        discoverVariablesExtension: "dnsvars.<external-config-name>"
```

On the cluster resource then specify desired DNS image repo:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: dns
        value:
          imageRepository: a.b.c.example.com
```

Applying this configuration will result in the DNS image repository being correctly set in the
`KubeadmControlPlaneTemplate`.

This hook is enabled by default, and can be explicitly disabled by omitting the `DNSVars`
and `DNSPatch` hook from the `--runtimehooks.enabled-handlers` flag.

If deploying via Helm, then this can be disabled by setting `handlers.DNSVars.enabled=false` and
`handlers.DNSPatch.enabled=false`.
