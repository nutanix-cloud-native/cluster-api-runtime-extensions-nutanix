---
title: "Extra API Server Certificate SANs"
---

If the API server can be accessed by alternative DNS addresses then setting additional SANs on the API server
certificate is necessary in order for clients to successfully validate the API server certificate.

To enable the API server certificate SANs enable the `extraapiservercertsansvars` and `extraapiservercertsanspatch`
external patches on `ClusterClass`.

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: ClusterClass
metadata:
  name: <NAME>
spec:
  patches:
    - name: apiserver-cert-sans
      external:
        generateExtension: "extraapiservercertsanspatch.<external-config-name>"
        discoverVariablesExtension: "extraapiservercertsansvars.<external-config-name>"
```

On the cluster resource then specify desired certificate SANs values:

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: <NAME>
spec:
  topology:
    variables:
      - name: extraAPIServerCertSANs
        value:
          - a.b.c.example.com
          - d.e.f.example.com
```

Applying this configuration will result in the certificate SANs being correctly set in the
`KubeadmControlPlaneTemplate`.

This hook is enabled by default, and can be explicitly disabled by omitting the `ExtraAPIServerCertSANsVars`
and `ExtraAPIServerCertSANsPatch` hook from the `--runtimehooks.enabled-handlers` flag.

If deploying via Helm, then this can be disabled by setting `handlers.ExtraAPIServerCertSANsVars.enabled=false` and
`handlers.ExtraAPIServerCertSANsPatch.enabled=false`.
