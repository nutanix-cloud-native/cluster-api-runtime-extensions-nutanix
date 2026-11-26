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
rejects configurations where any `jwt[*].issuer.url` matches
`clusterConfig.serviceAccountIssuer.issuer`, an explicit
`--service-account-issuer` flag, or kubeadm's default
`https://kubernetes.default.svc.<serviceDomain>` (when no ServiceAccount issuer
is configured), where `ExtraArgs` already
contains `--authentication-config` with a value other than
`/etc/kubernetes/authentication.yaml`, or where `APIServer.ExtraVolumes`
defines a conflicting `authentication-config` volume or
`/etc/kubernetes/authentication.yaml` mount.

See [Migrating from `KubeadmControlPlaneTemplate` authentication](#migrating-from-kubeadmcontrolplanetemplate-authentication)
for the step-by-step migration procedure.

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

## Migrating from `KubeadmControlPlaneTemplate` authentication

Existing clusters that configured API server authentication directly in a
`KubeadmControlPlaneTemplate` — either through legacy `--oidc-*` flags or
through a hand-managed `AuthenticationConfiguration` file — can migrate to
CAREN-managed `clusterConfig.authentication` in a single control-plane rollout
without an intermediate unauthenticated state.

### Field translation checklist

When translating `--oidc-*` flags or a manual `AuthenticationConfiguration`
into `clusterConfig.authentication`:

- **`prefix` is required when `claim` is set** on `claimMappings.username` or
  `claimMappings.groups`. Set `prefix: ""` to disable prefixing; `prefix: "-"`
  prepends a literal `"-"` (unlike legacy `--oidc-username-prefix="-"`, which
  disabled prefixing).
- **`--oidc-ca-file` (or a file-mounted CA) becomes inline PEM** in
  `issuer.certificateAuthority` rather than a host file path.
- **Multiple audiences require `audienceMatchPolicy: MatchAny`** on `issuer`.
- **`issuer.url` must not match `clusterConfig.serviceAccountIssuer.issuer` or `--service-account-issuer`**
  (or kubeadm's default `https://kubernetes.default.svc.<serviceDomain>` when
  no ServiceAccount issuer is configured).

### Step-by-step migration

Because `KubeadmControlPlaneTemplate.spec.template.spec` is immutable in
Cluster API and CAREN's `/validate-cluster` webhook rejects
`clusterConfig.authentication` while the referenced
`KubeadmControlPlaneTemplate` still carries `--oidc-*` flags or conflicting
`--authentication-config` / `extraVolumes` entries, follow this sequence:

1. **Pause the target `Cluster`s** (`spec.paused: true`) so Cluster API does not
   reconcile an intermediate `KubeadmControlPlane` without authentication while
   you swap templates. If multiple `Cluster`s share the `ClusterClass`, pause
   and migrate them together so none reconcile against the clean template before
   their `clusterConfig.authentication` variable is set:

   ```shell
   kubectl patch cluster <CLUSTER_NAME> -n <NAMESPACE> --type=merge \
     -p '{"spec":{"paused":true}}'
   ```

2. **Create a new `KubeadmControlPlaneTemplate`** (with a new `metadata.name`)
   copied from the existing template, removing the manual authentication
   entries:
   - Remove all `oidc-*` or `authentication-config` entries from
     `spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraArgs`.
   - Remove any manual authentication config or OIDC CA volume mount from
     `spec.template.spec.kubeadmConfigSpec.clusterConfiguration.apiServer.extraVolumes`.
   - Remove any manual authentication config or OIDC CA file entry from
     `spec.template.spec.kubeadmConfigSpec.files`.

3. **Update `ClusterClass.spec.controlPlane.templateRef.name`** to reference the
   new `KubeadmControlPlaneTemplate` before updating the `Cluster`:

   ```shell
   kubectl patch clusterclass <CLUSTERCLASS_NAME> -n <CLASS_NAMESPACE> --type=merge \
     -p '{"spec":{"controlPlane":{"templateRef":{"name":"<NEW_KCP_TEMPLATE_NAME>"}}}}'
   ```

4. **Set `clusterConfig.authentication` on each paused `Cluster`** under
   `spec.topology.variables`. Because the `ClusterClass` now points at the clean
   `KubeadmControlPlaneTemplate`, CAREN's validating webhook validates the new
   `authentication` settings with no template conflicts.

5. **Unpause each `Cluster`** (`spec.paused: false`):

   ```shell
   kubectl patch cluster <CLUSTER_NAME> -n <NAMESPACE> --type=merge \
     -p '{"spec":{"paused":false}}'
   ```

   Cluster API reconciles `KubeadmControlPlane` directly from the previous spec
   to the CAREN-managed `/etc/kubernetes/authentication.yaml` configuration and
   performs a single rolling replacement of control-plane machines (or no
   rollout at all if the previous manual setup already produced an identical
   file, volume mount, and `--authentication-config` argument at
   `/etc/kubernetes/authentication.yaml`).

6. **Delete the retired `KubeadmControlPlaneTemplate`** once no `ClusterClass`
   references it.

## Updating issuer CA certificates

### Planned CA rotation (declarative)

`issuer.certificateAuthority` accepts a PEM bundle containing multiple
certificates. For planned identity-provider CA rotations:

1. Concatenate both the current and incoming CA certificates into
   `clusterConfig.authentication.jwt[*].issuer.certificateAuthority` ahead of
   rotating the identity provider's TLS certificate; Cluster API rolls
   control-plane machines so `kube-apiserver` trusts both CAs.
2. Switch the identity provider's TLS certificate to the new CA.
3. Remove the retired CA from `issuer.certificateAuthority` during a subsequent
   maintenance window.

### Out-of-band CA update without an immediate control-plane rollout

If an identity provider's CA must be updated urgently outside a maintenance
window without immediately replacing control-plane machines, you must keep the
management-cluster spec and the on-disk `/etc/kubernetes/authentication.yaml`
aligned so neither overwrites the other:

- Updating `clusterConfig.authentication` on an unpaused control plane triggers
  an immediate `KubeadmControlPlane` machine rollout.
- Updating `/etc/kubernetes/authentication.yaml` only on the control-plane hosts
  while leaving `clusterConfig.authentication` unchanged leaves the management
  cluster stale: any subsequent cluster lifecycle event (scale-out, Kubernetes
  upgrade, or `MachineHealthCheck` replacement) will provision nodes with the
  old CA and overwrite the out-of-band update on new machines.

To apply the CA update immediately and defer the control-plane rollout without
conflicts or unintended overwrites:

1. **Pause `KubeadmControlPlane` reconciliation** by adding the Cluster API
   `cluster.x-k8s.io/paused` annotation to the `KubeadmControlPlane` resource.
   Cluster API's topology controller uses Server-Side Apply and preserves this
   annotation while continuing to reconcile `KubeadmControlPlane.spec` and
   keeping worker `MachineDeployment`s and addons active:

   ```shell
   KCP_NAME=$(kubectl get cluster <CLUSTER_NAME> -n <NAMESPACE> \
     -o jsonpath='{.spec.controlPlaneRef.name}')
   kubectl annotate kubeadmcontrolplane "${KCP_NAME}" -n <NAMESPACE> \
     cluster.x-k8s.io/paused="" --overwrite
   ```

   > **Note:** While `cluster.x-k8s.io/paused` is set on `KubeadmControlPlane`,
   > Cluster API will not scale, upgrade, or remediate control-plane machines.
   > Remove the annotation during the next maintenance window.

2. **Update `clusterConfig.authentication.jwt[*].issuer.certificateAuthority` on
   the `Cluster`** first. CAREN validates the updated PEM bundle at admission
   and patches `KubeadmControlPlane.spec.kubeadmConfigSpec.files` with the new
   rendered `/etc/kubernetes/authentication.yaml`, while `KubeadmControlPlane`
   holds off replacing existing control-plane machines.

3. **Extract the CAREN-rendered file from `KubeadmControlPlane` and write it in
   place on each control-plane node**. Extracting the rendered content from
   `KubeadmControlPlane` guarantees that the host file is byte-for-byte
   identical to CAREN's desired state and preserves all other authenticators and
   rules:

   ```shell
   kubectl get kubeadmcontrolplane "${KCP_NAME}" -n <NAMESPACE> \
     -o jsonpath='{.spec.kubeadmConfigSpec.files[?(@.path=="/etc/kubernetes/authentication.yaml")].content}' \
     > authentication.yaml
   ```

   Copy `authentication.yaml` to each control-plane host and **overwrite
   `/etc/kubernetes/authentication.yaml` in place** (keeping `0600`
   permissions):

   ```shell
   sudo chmod 0600 /etc/kubernetes/authentication.yaml
   sudo tee /etc/kubernetes/authentication.yaml < authentication.yaml >/dev/null
   ```

   > **Important:** Do **not** replace `/etc/kubernetes/authentication.yaml` via
   > atomic rename (`mv`, `sed -i`, or editors that write a temporary file and
   > rename). CAREN mounts `/etc/kubernetes/authentication.yaml` into the
   > `kube-apiserver` static pod with `hostPath` `PathType: File` (a single-file
   > bind mount). Renaming over the path creates a new host inode while the
   > running `kube-apiserver` container remains bound to the old unlinked inode,
   > preventing `kube-apiserver`'s ~1-minute automatic reload from seeing the
   > updated CA until the pod restarts. Writing in place updates the mounted
   > inode directly so `kube-apiserver` reloads the new CA within about a minute
   > with no API server restart.

4. **Unpause `KubeadmControlPlane` during the next maintenance window**:

   ```shell
   kubectl annotate kubeadmcontrolplane "${KCP_NAME}" -n <NAMESPACE> \
     cluster.x-k8s.io/paused-
   ```

   Because `KubeadmControlPlane.spec.kubeadmConfigSpec.files` already contains
   the identical updated `AuthenticationConfiguration`, unpausing rolls the
   control-plane machines onto the same configuration without reverting or
   overwriting the CA update.

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
