+++
title = "Encryption At REST"
+++

`encryptionAtRest` variable enables encrypting kubernetes resources at REST using provided encryption provider.
When this variable is set, kuberntetes secrets and configmaps are encrypted before writing them at `etcd`.

If the `encryptionAtRest` property is not specified, then
the customization will be skipped. The secrets and configmaps will not be stored as encrypted in `etcd`.

We support following encryption providers
- aescbc
- secretbox

More information about encryption at REST: https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/

## Example

To encrypt configmaps and secrets for using `aescbc` and `secretbox` encryption providers:

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
          encryptionAtRest:
            providers:
              - aescbc: {}
              - secretbox: {}
```

Applying this configuration will result in `<CLUSTER_NAME>-encryption-config` secret generated and following value being set:

- `KubeadmControlPlaneTemplate`:

  - ```yaml
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          apiServer:
            extraArgs:
              encryption-provider-config: /etc/kubernetes/pki/encryptionconfig.yaml
      files:
        - contentFrom:
            secret:
              key: config
              name: my-cluster-encryption-config
          path: /etc/kubernetes/pki/encryptionconfig.yaml
          permissions: "0640"          
    ```
