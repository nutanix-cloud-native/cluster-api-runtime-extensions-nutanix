+++
title = "Service Account Issuer"
+++

<!---
Copyright 2026 Nutanix. All rights reserved.
SPDX-License-Identifier: Apache-2.0
--->

This customization allows configuration of the Kubernetes API server's ServiceAccount
token issuer for workload identity. When specified, CAREN configures the API server
with `--service-account-issuer`, and optionally `--api-audiences`,
`--service-account-jwks-uri`, and `--service-account-max-token-expiration`, via
`KubeadmControlPlaneTemplate`.

This is an opt-in feature. When `serviceAccountIssuer` is not specified in the cluster
configuration, no ServiceAccount issuer configuration is applied and existing clusters are
unaffected.

This feature is available for **kubeadm-based providers only** (AWS, Docker, Nutanix).
EKS is excluded because it does not expose API server flags through `ManagedControlPlane`.

The API server serves OIDC discovery at `/.well-known/openid-configuration` and JWKS at
`/openid/v1/jwks` based on the configured issuer. Tokens are issued with the configured
`iss` claim.

## Kubernetes version requirement

Available on all Kubernetes versions supported by CAREN for kubeadm-based clusters
(bound ServiceAccount token volumes and ServiceAccount issuer OIDC discovery have been GA
since Kubernetes 1.21).

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `issuer` | `string` (`https://`) | Yes | Identifier asserted in ServiceAccount JWT `iss` claims. Must be an `https://` URL (max 2048 characters) with a non-empty hostname and valid port, without userinfo, query parameters, or fragments. Must not match any `clusterConfig.authentication.jwt[].issuer.url`. Triggers `--service-account-issuer`. |
| `audiences` | `[]string` | No | List of audiences tokens are bound to (max 50 unique entries, each 1–253 characters, without whitespace, commas, or double quotes). When set, rendered as comma-joined `--api-audiences` sorted alphabetically. When absent, `--api-audiences` is omitted and the API server defaults to `[issuer]`. **Warning:** setting only external audiences (such as `sts.amazonaws.com`) without keeping the cluster's in-cluster API audience (`issuer` or `https://kubernetes.default.svc.<serviceDomain>`), or removing a previously accepted audience, causes existing in-cluster ServiceAccount tokens bound to the omitted audience to be rejected until refreshed. |
| `jwksURI` | `string` (`https://`) | No | Override for the `jwks_uri` advertised in `/.well-known/openid-configuration` (max 2048 characters). Must be an `https://` URL with a non-empty hostname and valid port, without userinfo or fragments (query parameters are allowed per OpenID Connect Discovery 1.0 Section 3). Triggers `--service-account-jwks-uri`. |
| `maxTokenExpiration` | `duration` | No | Maximum lifetime for ServiceAccount tokens as a Go duration between `1h` and `1193046h28m16s` (`2^32` seconds), e.g. `2h` or `24h` (`kube-apiserver` rejects values below `1h` and logs a startup warning at `<= 3607s`). Triggers `--service-account-max-token-expiration`. Note that `kube-apiserver` enables `--service-account-extend-token-expiration=true` by default, which extends default admission-injected Pod projected tokens (`3607s`) up to 1 year unless a Pod's `serviceAccountToken` volume projection sets `expirationSeconds` explicitly. |

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

When overriding `audiences`, include the cluster's Kubernetes API audience
(for example `https://kubernetes.default.svc.cluster.local`) alongside external
relying-party audiences if workloads also authenticate against the Kubernetes API
using that audience:

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
              - https://kubernetes.default.svc.cluster.local
```

This renders `api-audiences` as
`https://kubernetes.default.svc.cluster.local,sts.amazonaws.com` (sorted
alphabetically regardless of input order).

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

### Issuer with maximum token expiration

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
            maxTokenExpiration: 2h
```

## Notes

- **Rollout and token rotation:** Changing `serviceAccountIssuer` updates
  `KubeadmControlPlane` and triggers a rolling replacement of control-plane machines.
  Because CAREN sets a single `--service-account-issuer`, changing `issuer` (or
  removing an entry from `audiences`) causes updated API servers to immediately
  reject existing ServiceAccount tokens carrying the previous `iss` or removed `aud`
  with `401 Unauthorized` until kubelet refreshes projected volume tokens (at ~80%
  of token lifetime) or Pods are restarted. Plan Pod restarts when rotating `issuer`
  or narrowing `audiences` on running clusters. CAREN's Cluster validating webhook
  emits an admission warning when `audiences` omits both `issuer` and
  `https://kubernetes.default.svc.<serviceDomain>`, or when an update removes a
  previously accepted audience.
- **Conflict and idempotency:** Existing `apiServer.extraArgs` entries that already
  match the desired values are preserved idempotently. If `service-account-issuer`,
  `api-audiences`, `service-account-jwks-uri`, or `service-account-max-token-expiration`
  are already set directly via `apiServer.extraArgs` with a different or `nil` value
  (or when the corresponding optional field is unset), the handler returns an error
  directing you to remove the direct `extraArgs` and use
  `clusterConfig.serviceAccountIssuer` instead.
- **Composition:** This feature composes additively with `podSecurityAdmission`,
  `encryptionAtRest`, and `authentication` (`AuthenticationConfiguration`). When
  used together with `clusterConfig.authentication`, `serviceAccountIssuer.issuer`
  must not equal any `authentication.jwt[].issuer.url` because `kube-apiserver`
  forbids external JWT authenticators from overlapping the ServiceAccount issuer.

## Removing ServiceAccount issuer configuration

When `serviceAccountIssuer` is unset or omitted, the handler emits no patches
against the `ClusterClass` `KubeadmControlPlaneTemplate`. Clusters that never set
`serviceAccountIssuer` are unaffected, while removing `serviceAccountIssuer` from a
cluster that previously enabled it reconciles `KubeadmControlPlane` back to the
unpatched `ClusterClass` template — removing the managed ServiceAccount issuer flags
and rolling control-plane machines back to kubeadm's default issuer. As with
rotating `issuer`, Pods holding tokens minted with the removed custom `iss` will need
projected token refresh or restart once control-plane nodes roll.

## See also

- [Kubernetes ServiceAccount token volume projection](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#serviceaccount-token-volume-projection)
- [Configure ServiceAccount issuer discovery](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-issuer-discovery)
