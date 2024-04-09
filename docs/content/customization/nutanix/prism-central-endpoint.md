+++
title = "Prism Central Endpoint"
+++

Configure Prism Central Endpoint to create machines on.

## Examples

### Set Prism Central Endpoint

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
            prismCentralEndpoint:
              credentials:
                name: secret-name
              url: https://x.x.x.x:9440
              insecure: false
```

Applying this configuration will result in the following value being set:

- `NutanixClusterTemplate`:

```yaml
spec:
  template:
    spec:
      prismCentral:
        address: x.x.x.x
        insecure: false
        port: 9440
        credentialRef:
          kind: Secret
          name: secret-name
```

### Provide an Optional Trusted CA Bundle

If the Prism Central endpoint uses a self-signed certificate, you can provide an additional trust bundle
to be used by the Nutanix provider.
This is a base64 PEM encoded x509 cert for the RootCA that was used to create the certificate for a Prism Central

See [Nutanix Security Guide] for more information.

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
            prismCentralEndpoint:
              # ...
              additionalTrustBundle: "LS0...="
```

Applying this configuration will result in the following value being set:

- `NutanixClusterTemplate`:

```yaml
spec:
  template:
    spec:
      prismCentral:
        # ...
        additionalTrustBundle:
          kind: String
          data: |-
            -----BEGIN CERTIFICATE-----
            ...
            -----END CERTIFICATE-----
```

[Nutanix Security Guide]: https://portal.nutanix.com/page/documents/details?targetId=Nutanix-Security-Guide-v6_5:mul-security-ssl-certificate-pc-t.html
