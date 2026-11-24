+++
title = "Authentication"
+++

<!---
Copyright 2026 Nutanix. All rights reserved.
SPDX-License-Identifier: Apache-2.0
--->

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

Requires **Kubernetes >= 1.34** (`apiserver.config.k8s.io/v1` went GA in 1.34;
1.29–1.33 only served `v1beta1`, which CAREN does not emit). When
`clusterConfig.authentication` is set on a cluster whose Kubernetes version is
older than `1.34.0`, both the `Cluster` validating webhook and the topology
mutation handler reject the configuration rather than rendering an
`AuthenticationConfiguration` file that the API server cannot parse.

## Configuration

Each entry under `clusterConfig.authentication.jwt` mirrors an upstream
`apiserver.config.k8s.io/v1` `JWTAuthenticator`:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `issuer.url` | `string` (`https://`) | Yes | OIDC issuer URL (no query or fragment), unique across `jwt` entries and distinct from `--service-account-issuer`. Must match the `iss` claim in presented tokens. |
| `issuer.discoveryURL` | `string` (`https://`) | No | Overrides `{issuer.url}/.well-known/openid-configuration`. Must differ from `issuer.url` and be unique across `jwt` entries. |
| `issuer.certificateAuthority` | `string` (PEM) | No | Inline PEM-encoded CA certificate bundle used to verify TLS connections to the OIDC issuer. |
| `issuer.audiences` | `[]string` | Yes | Acceptable JWT audiences (non-empty, unique items). |
| `issuer.audienceMatchPolicy` | `""` \| `MatchAny` | When `len(audiences) > 1` | Must be set to `MatchAny` when more than one audience is configured. |
| `claimValidationRules` | `[]ClaimValidationRule` | No | Each rule sets either `claim` + `requiredValue` or a boolean CEL `expression` (with optional `message`). |
| `claimMappings.username` | `PrefixedClaimOrExpression` | Yes | Either `expression` (CEL) or `claim` + `prefix` (`prefix: ""` disables prefixing; `"-"` is a literal prefix). |
| `claimMappings.groups` | `PrefixedClaimOrExpression` | No | Either `expression` (CEL) or `claim` + `prefix` (`prefix: ""` disables prefixing; `"-"` is a literal prefix). |
| `claimMappings.uid` | `ClaimOrExpression` | No | Either `claim` or `expression` (CEL). |
| `claimMappings.extra` | `[]ExtraMapping` | No | Unique lowercase domain-prefixed `key` + CEL `valueExpression` pairs (`k8s.io`, `kubernetes.io`, and their subdomains are reserved). |
| `userValidationRules` | `[]UserValidationRule` | No | Boolean CEL `expression` rules (with optional `message`) evaluated against the mapped `user` object. |

## Mutual exclusivity with `--oidc-*` flags

The file-based `AuthenticationConfiguration` and the legacy `--oidc-*` API server
flags (`--oidc-issuer-url`, `--oidc-client-id`, `--oidc-ca-file`,
`--oidc-username-claim`, `--oidc-groups-claim`, `--oidc-required-claim`, etc.)
are **mutually exclusive**. The API server will refuse to start if both are
present.

CAREN enforces these constraints at `Cluster` admission (in the validating
webhook) and during topology reconciliation: if `APIServer.ExtraArgs` already
contains any flag whose name starts with `oidc-` and `clusterConfig.authentication`
is also set, CAREN rejects the configuration and instructs the operator to
remove the legacy flags and use `authentication` instead. Likewise, CAREN
rejects configurations where any `jwt[*].issuer.url` matches an explicit
`--service-account-issuer` flag (or kubeadm's default
`https://kubernetes.default.svc.<serviceDomain>`), where `ExtraArgs` already
contains `--authentication-config` with a value other than
`/etc/kubernetes/authentication.yaml`, or where `APIServer.ExtraVolumes`
defines a conflicting `authentication-config` volume or
`/etc/kubernetes/authentication.yaml` mount.

To migrate from flag-based OIDC, remove all `--oidc-*` entries from
`KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` and express the
same intent through `clusterConfig.authentication`:

- **`prefix` is required when `claim` is set** on `claimMappings.username` or
  `claimMappings.groups`. Set `prefix: ""` to disable prefixing; `prefix: "-"`
  prepends a literal `"-"` (unlike legacy `--oidc-username-prefix="-"`, which
  disabled prefixing).
- **`--oidc-ca-file` becomes inline PEM** in `issuer.certificateAuthority`
  rather than a host file path.
- **Multiple audiences require `audienceMatchPolicy: MatchAny`** on `issuer`.

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
                    prefix: ""
                  groups:
                    claim: groups
                    prefix: ""
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
                    prefix: ""
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
                  - expression: 'claims.email_verified == true'
                    message: email must be verified
                claimMappings:
                  username:
                    expression: 'claims.email + ":ext"'
                  groups:
                    claim: groups
                    prefix: ""
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
        prefix: ""
      groups:
        claim: groups
        prefix: ""
```

For the multi-issuer and CEL examples the corresponding fields (`discoveryURL`,
`certificateAuthority`, `audienceMatchPolicy`, `claimValidationRules`,
`claimMappings.extra`, `userValidationRules`) appear in the same positions with
the values provided in the CAREN variable.

kube-apiserver re-reads this file about once a minute. CAREN does not edit the
file in place. Enabling or changing `authentication` updates the
KubeadmControlPlane spec (the file and the `--authentication-config` argument),
and Cluster API replaces control-plane machines for that change.

## Removing authentication configuration

When `authentication` is unset or omitted, the handler emits no patches against
the `ClusterClass` `KubeadmControlPlaneTemplate`. Clusters that never set
`authentication` are unaffected, while removing `authentication` from a cluster
that previously enabled it reconciles `KubeadmControlPlane` back to the
unpatched `ClusterClass` template — removing `/etc/kubernetes/authentication.yaml`,
its volume mount, and `--authentication-config`, and rolling control-plane
machines.

## See also

- [Kubernetes Authentication Configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration)
- [KEP-3331: Structured Authentication Configuration](https://github.com/kubernetes/enhancements/blob/master/keps/sig-auth/3331-structured-authentication-configuration/README.md)
