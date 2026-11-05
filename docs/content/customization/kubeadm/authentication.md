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

Requires **Kubernetes >= 1.34** (`apiserver.config.k8s.io/v1` went GA in 1.34;
1.29–1.33 only served `v1beta1`, which CAREN does not emit). The API server will
fail to parse the file on older versions. CAREN does not gate on
`Cluster.spec.topology.version`; document the minimum and let the API server fail
fast.

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
contains `--authentication-config` with a value other than the expected path
`/etc/kubernetes/authentication.yaml`, the handler returns an error rather than
silently overwriting a user-managed path.

To migrate from flag-based OIDC, remove all `--oidc-*` entries from
`KubeadmConfigSpec.ClusterConfiguration.APIServer.ExtraArgs` and express the
same intent through `clusterConfig.authentication`.

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

Setting `authentication` back to absent (or removing the variable) is a no-op on
the next reconciliation — the handler emits no patches, consistent with other
handlers.

## See also

- [Kubernetes Authentication Configuration](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#using-authentication-configuration)
- [KEP-3331: Structured Authentication Configuration](https://github.com/kubernetes/enhancements/blob/master/keps/sig-auth/3331-structured-authentication-configuration/README.md)
