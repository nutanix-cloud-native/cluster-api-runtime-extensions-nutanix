---
title: "Extra API Server Certificate SANs"
---

If the API server can be accessed by alternative DNS addresses then setting additional SANs on the API server
certificate is necessary in order for clients to successfully validate the API server certificate.

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

On the cluster resource then specify desired certificate SANs values:

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
          extraAPIServerCertSANs:
            - a.b.c.example.com
            - d.e.f.example.com
```

Applying this configuration will result in the certificate SANs being correctly set in the
`KubeadmControlPlaneTemplate`.
