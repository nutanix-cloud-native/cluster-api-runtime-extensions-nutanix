+++
title = "Control Plane Endpoint"
+++

Configure Control Plane Endpoint. Defines the host IP and port of the CAPX Kubernetes cluster.

## Examples

### Set Control Plane Endpoint

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
          nutanix:
            controlPlaneEndpoint:
              host: x.x.x.x
              port: 6443
              virtualIP: {}
```

Applying this configuration will result in the following value being set:

- `NutanixCluster`:

```yaml
spec:
  template:
    spec:
      controlPlaneEndpoint:
        host: x.x.x.x
        port: 6443
```

- `KubeadmControlPlaneTemplate`

```yaml
  spec:
    kubeadmConfigSpec:
      files:
      - content: |
          apiVersion: v1
          kind: Pod
          metadata:
            name: kube-vip
            namespace: kube-system
          spec:
          ...
        owner: root:root
        path: /etc/kubernetes/manifests/kube-vip.yaml
        permissions: "0600"
      postKubeadmCommands:
        # Only added for clusters version >=v1.29.0
        - |-
          if [ -f /run/kubeadm/kubeadm.yaml ]; then
            sed -i 's#path: /etc/kubernetes/super-admin.conf#path: ...
          fi
      preKubeadmCommands:
        # Only added for clusters version >=v1.29.0
        - |-
          if [ -f /run/kubeadm/kubeadm.yaml ]; then
            sed -i 's#path: /etc/kubernetes/admin.conf#path: ...
          fi
```
