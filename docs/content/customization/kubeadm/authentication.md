<!---
Copyright 2026 Nutanix. All rights reserved.
SPDX-License-Identifier: Apache-2.0
--->

+++
title = "Authentication"
+++

This customization allows configuration of the Kubernetes API server's
OIDC/JWT authentication through
[Structured Authentication Configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration)
(KEP-3331). When `authentication` is specified, CAREN renders an
`AuthenticationConfiguration` file (`apiserver.config.k8s.io/v1`) at
`/etc/kubernetes/authentication.yaml`, mounts it into the API server, and sets
`--authentication-config` to that path.

This is an opt-in feature. When `authentication` is not specified, no
authentication configuration file is rendered and existing clusters are
unaffected.

## Kubernetes version requirement

Structured Authentication Configuration was introduced as **beta in Kubernetes
1.29** and went **GA in 1.31**. The `--authentication-config` flag is not
recognised on clusters older than 1.29. CAREN does not gate on
`Cluster.spec.topology.version`; the configuration is harmless to generate
regardless of version, but the API server will fail to start on unsupported
versions.

> **Recommended:** Use Kubernetes **1.31 or later** (GA, enabled by default).

## Mutual exclusivity with `--oidc-*` flags

The file-based `AuthenticationConfiguration` and the legacy `--oidc-*` API server
flags (`--oidc-issuer-url`, `--oidc-client-id`, `--oidc-ca-file`,
`--oidc-username-claim`, `--oidc-groups-claim`, `--oidc-required-claim`, etc.)
are **mutually exclusive**. The API server will refuse to start if both are
present.

CAREN enforces this at reconciliation time: if `APIServer.ExtraArgs` already
contains any flag whose name starts with `oidc-` and `clusterConfig.authentication`
is also set, the handler returns an error instructing the operator to remove the
legacy flags and use `authentication` instead. Likewise, if `ExtraArgs` already
contains `--authentication-config` with a value other than the well-known path
`/etc/kubernetes/authentication.yaml`, the handler returns an error rather than
silently overwriting a user-managed path.

To migrate from flag-based OIDC, remove all `--oidc-*` entries from
`KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` and express the
same intent through `clusterConfig.authentication`.

## Configuration

`authentication` is set under `clusterConfig` and contains an ordered list of
JWT authenticators. Order is the API server's evaluation order and is preserved
exactly as declared.

### Top-level

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `jwt` | `[]JWTAuthenticator` (1–32) | Yes | Ordered list of JWT authenticators. Issuer URLs must be unique across the list. |

### Issuer (`jwt[].issuer`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | `string` (1–2048, `https://` URL) | Yes | OIDC issuer URL. Must be unique within the `jwt` list. |
| `audiences` | `[]string` (1–32, 1–256 each, unique) | Yes | Audiences the token must contain. Order is preserved. |
| `discoveryURL` | `string` | No | Override for the OIDC discovery URL. Defaults to `{url}/.well-known/openid-configuration`. |
| `certificateAuthority` | `string` | No | PEM-encoded CA bundle for TLS verification of the issuer. Inlined in the YAML, not mounted as a separate file. |
| `audienceMatchPolicy` | `""` \| `"MatchAny"` | No | Audience matching policy. Empty (default) requires all audiences to match; `MatchAny` requires at least one. |

### ClaimValidationRules (`jwt[].claimValidationRules`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `claim` | `string` | One of `claim`+`requiredValue` **or** `expression` | JWT claim name to validate. |
| `requiredValue` | `string` | With `claim` | Value the claim must equal. |
| `expression` | `string` (CEL) | One of | CEL expression evaluated against `claims`. See [CEL example](#cel-expression-example) below. |
| `message` | `string` | No | Human-readable message when validation fails. For `expression` rules this is surfaced as the error detail. |

Each entry must be **either** `{claim + requiredValue}` **or** `{expression}`
(not both, not neither). Up to 32 rules per authenticator.

### ClaimMappings (`jwt[].claimMappings`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `username` | `PrefixedClaimOrExpression` | Yes | How to derive the Kubernetes username from the token. |
| `groups` | `ClaimOrExpression` | No | How to derive group memberships. |
| `uid` | `ClaimOrExpression` | No | How to derive the UID. |
| `extra` | `[]ExtraMapping` (0–32) | No | Additional mappings into `userInfo.extra`. |

#### PrefixedClaimOrExpression (`claimMappings.username`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `claim` | `string` | One of `claim`+`prefix` **or** `expression` | JWT claim name. |
| `prefix` | `string` \| `nil` | With `claim` | Prefix added to the claim value. `nil`/absent → default `<issuer.url>#`; `"-"` → no prefix. |
| `expression` | `string` (CEL) | One of | CEL expression to compute the value (e.g. `claims.email + ":ext"`). When set, `claim` and `prefix` must not be set. |

#### ClaimOrExpression (`claimMappings.groups`, `claimMappings.uid`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `claim` | `string` | One of `claim` **or** `expression` | JWT claim name. |
| `expression` | `string` (CEL) | One of | CEL expression to compute the value. When set, `claim` must not be set. |

#### ExtraMapping (`claimMappings.extra[]`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | `string` | Yes | Key in `userInfo.extra` where the result is stored. |
| `valueExpression` | `string` (CEL) | Yes | CEL expression whose result populates the key. |

### UserValidationRules (`jwt[].userValidationRules`)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `expression` | `string` (CEL) | Yes | CEL expression validated against the derived `userInfo`. |
| `message` | `string` | No | Message surfaced when the expression evaluates to `false`. |

Up to 32 rules per authenticator.

## Examples

### Single-issuer OIDC

The most common case — one identity provider, standard claim mappings:

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
          authentication:
            jwt:
              - issuer:
                  url: https://issuer.example.com
                  audiences:
                    - k8s
                claimMappings:
                  username:
                    claim: email
                  groups:
                    claim: groups
```

Applying this configuration will result in the following being applied to the
`KubeadmControlPlaneTemplate`:

- A file at `/etc/kubernetes/authentication.yaml` with an
  `AuthenticationConfiguration` document containing the declared authenticator
- A volume mount for that file
- The `--authentication-config=/etc/kubernetes/authentication.yaml` API server
  extra arg

### Multi-issuer

Multiple authenticators are evaluated in order. Each issuer URL must be unique:

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
          authentication:
            jwt:
              - issuer:
                  url: https://issuer-a.example.com
                  audiences:
                    - k8s-a
                claimMappings:
                  username:
                    claim: email
                    prefix: a.example.com#
                  groups:
                    claim: groups
              - issuer:
                  url: https://issuer-b.example.com
                  audiences:
                    - k8s-b
                  audienceMatchPolicy: MatchAny
                claimMappings:
                  username:
                    claim: sub
                    prefix: b.example.com#
```

The rendered file preserves the declared order (`issuer-a` first,
`issuer-b` second), which is the API server's evaluation order.

### CEL expression example

CEL expressions can be used for claim validation, claim mapping, and user
validation. This example combines several CEL features:

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
          authentication:
            jwt:
              - issuer:
                  url: https://issuer.example.com
                  audiences:
                    - k8s
                    - k8s-internal
                  audienceMatchPolicy: MatchAny
                claimValidationRules:
                  - claim: hd
                    requiredValue: example.com
                  - expression: 'claims.exp - claims.nbf <= 86400'
                    message: token lifetime must not exceed 24 hours
                claimMappings:
                  username:
                    expression: 'claims.email + ":ext"'
                  groups:
                    claim: groups
                  uid:
                    claim: sub
                  extra:
                    - key: example.com/tenant
                      valueExpression: 'claims.tenant_id'
                userValidationRules:
                  - expression: '!user.username.startsWith("system:")'
                    message: username cannot be privileged
```

### Full YAML — rendered `AuthenticationConfiguration`

The handler renders a single document at `/etc/kubernetes/authentication.yaml`.
For the single-issuer example above the generated file looks like:

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: AuthenticationConfiguration
jwt:
  - issuer:
      url: https://issuer.example.com
      audiences:
        - k8s
    claimMappings:
      username:
        claim: email
```

For the multi-issuer and CEL examples the corresponding fields (`discoveryURL`,
`certificateAuthority`, `audienceMatchPolicy`, `claimValidationRules`,
`claimMappings.extra`, `userValidationRules`) appear in the same positions with
the values provided in the CAREN variable.

The file is polled by the API server every minute and hot-reloaded on change;
no API server restart is needed after CAREN updates the file.

## Removing authentication configuration

Setting `authentication` back to absent (or removing the variable) reconciles the
`KubeadmControlPlaneTemplate` by removing the file, its volume mount, and the
`--authentication-config` flag. Legacy `--oidc-*` flags can then be used again
if desired.

## See also

- [Kubernetes Authentication Configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration)
- [KEP-3331: Structured Authentication Configuration](https://github.com/kubernetes/enhancements/blob/master/keps/sig-auth/3331-structured-authentication-configuration/README.md)
