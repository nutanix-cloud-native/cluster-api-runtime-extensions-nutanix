+++
title = "Encryption At Rest"
+++

`encryptionAtRest` variable enables encrypting kubernetes resources at rest using provided encryption provider.
When this variable is set, kuberntetes `secrets` and `configmap`s are encrypted before writing them at `etcd`.

If the `encryptionAtRest` property is not specified, then
the customization will be skipped. The `secrets` and `configmaps` will not be stored as encrypted in `etcd`.

We support following encryption providers

- aescbc
- secretbox

More information about encryption at-rest: [Encrypting Confidential Data at Rest
](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/)

## Example

To encrypt `configmaps` and `secrets` kubernetes resources using `aescbc` encryption provider:

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
```

Applying this configuration will result in

1. `<CLUSTER_NAME>-encryption-config` secret generated.

  A secret key for the encryption provider is generated and stored in `<CLUSTER_NAME>-encryption-config` secret.
  The APIServer will be configured to use the secret key to encrypt `secrets` and
   `configmaps` kubernetes resources before writing them to etcd.
  When reading resources from `etcd`, encryption provider that matches the stored data attempts in order to decrypt the data.
  CAREN currently does not rotate the key once it generated.

1. Configure APIServer with encryption configuration:

- `KubeadmControlPlaneTemplate`:

  ```yaml
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
              name: <CLUSTER_NAME>-encryption-config
          path: /etc/kubernetes/pki/encryptionconfig.yaml
          permissions: "0640"
  ```
