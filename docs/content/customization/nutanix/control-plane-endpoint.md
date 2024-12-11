+++
title = "Control Plane Endpoint"
+++

Configure Control Plane Endpoint. Defines the host IP and port of the Nutanix Kubernetes cluster.

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
            containers:
            - name: kube-vip
              args:
              - manager
              env:
              - name: port
                value: '6443'
              - name: address
                value: 'x.x.x.x'
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

### Set Control Plane Endpoint and a Different Virtual IP

It is also possible to set a separate virtual IP to be used by kube-vip from the control plane endpoint.
This is useful in VPC setups or other instances
when you have an external floating IP already associated with the virtual IP.

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
              virtualIP:
                configuration:
                  address: y.y.y.y
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
            containers:
            - name: kube-vip
              args:
              - manager
              env:
              - name: port
                value: '6443'
              - name: address
                value: 'y.y.y.y'
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
