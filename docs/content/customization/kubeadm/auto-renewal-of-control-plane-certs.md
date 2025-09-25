+++
title = "Auto-renewal of control plane certificates"
+++

`autoRenewCertificates` variable enables automatic renewal of control plane certificates by triggering a rollout of the
control plane nodes when the certificates on the control plane machines are about to expire.

More information about certificate renewal: [Automatically rotating certificates using Kubeadm Control Plane provider].

## Example

To enable automatic certificate renewal use the following configuration, applicable to all CAPI providers supported by
CAREN:

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
          controlPlane:
            autoRenewCertificates:
              daysBeforeExpiry: 30
```

Applying this configuration will result in the following configuration being applied:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      template:
        spec:
          rolloutBefore:
            certificatesExpiryDays: 30
    ```

[Automatically rotating certificates using Kubeadm Control Plane provider]: https://cluster-api.sigs.k8s.io/tasks/certs/auto-rotate-certificates-in-kcp.html
