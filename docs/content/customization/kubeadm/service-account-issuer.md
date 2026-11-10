+++
title = "Service Account Issuer"
+++

This customization allows configuration of the Kubernetes API server's ServiceAccount
token issuer for workload identity. When specified, CAREN configures the API server
with `--service-account-issuer`, and optionally `--api-audiences` and
`--service-account-jwks-uri`, via `KubeadmControlPlaneTemplate`.

This is an opt-in feature. When `serviceAccountIssuer` is not specified in the cluster
configuration, no ServiceAccount issuer configuration is applied and existing clusters are
unaffected.

This feature is available for **kubeadm-based providers only** (AWS, Docker, Nutanix).
EKS is excluded because it does not expose API server flags through `ManagedControlPlane`.

The API server serves OIDC discovery at `/.well-known/openid-configuration` and JWKS at
`openid/v1/jwks` based on the configured issuer. Tokens are issued with the configured
`iss` claim.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `issuer` | `string` | - | Required. Identifier asserted in ServiceAccount JWT `iss` claims. Must be an `https://` URL, max 2048 characters. Triggers `--service-account-issuer`. |
| `audiences` | `[]string` | `[]` | Optional. List of audiences tokens are bound to, max 50 entries, each 1-253 characters, unique. When set, rendered as comma-joined `--api-audiences` in declared order. When absent, the API server defaults to the issuer URL. |
| `jwksURI` | `string` | `""` | Optional. Override for the JWKS URI advertised in the discovery document, max 2048 characters. Must be an `https://` URL when set. Triggers `--service-account-jwks-uri`. |

## Examples

### Single issuer for workload identity

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
          serviceAccountIssuer:
            issuer: https://oidc.example.com/id/ABC123
```

Applying this configuration will result in the `KubeadmControlPlaneTemplate` containing:

```yaml
spec:
  template:
    spec:
      kubeadmConfigSpec:
        clusterConfiguration:
          apiServer:
            extraArgs:
              - name: service-account-issuer
                value: https://oidc.example.com/id/ABC123
```

### Issuer with audiences for cross-service validation

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
          serviceAccountIssuer:
            issuer: https://oidc.example.com
            audiences:
              - sts.amazonaws.com
              - k8s.example.com
```

This renders `api-audiences` as `sts.amazonaws.com,k8s.example.com` in declared order.

### Issuer with custom JWKS URI

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
          serviceAccountIssuer:
            issuer: https://oidc.example.com
            jwksURI: https://oidc.example.com/.well-known/jwks.json
```

## Notes

- **Rollout:** Changing the issuer triggers a control-plane rollout because the API server flags
  change. Tokens issued before the change remain valid until expiry.
- **Conflict:** If `service-account-issuer`, `api-audiences`, or `service-account-jwks-uri` are
  already set directly via `apiServer.extraArgs`, the handler returns an error directing you to
  remove the direct `extraArgs` and use `clusterConfig.serviceAccountIssuer` instead.
- **Composition:** This feature composes additively with `podSecurityAdmission`,
  `encryptionAtRest`, and `authentication` (`AuthenticationConfiguration`). They use different
  flags/files and do not conflict.
