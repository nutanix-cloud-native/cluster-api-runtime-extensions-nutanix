+++
title = "Pod Security Admission"
+++

This customization allows configuration of the
[Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/)
plugin with cluster-wide defaults. When specified, CAREN configures the API server to use the
`PodSecurity` admission plugin with the provided settings.

This is an opt-in feature. When `podSecurityAdmission` is not specified in the cluster
configuration, no Pod Security Admission configuration is applied and existing clusters are
unaffected.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enforce` | `privileged` \| `baseline` \| `restricted` | `privileged` | Level for enforce mode. Pods violating this level are rejected. |
| `audit` | `privileged` \| `baseline` \| `restricted` | `privileged` | Level for audit mode. Violations are recorded in the API server audit log. |
| `warn` | `privileged` \| `baseline` \| `restricted` | `privileged` | Level for warn mode. Violations trigger a user-facing warning. |
| `exemptions.namespaces` | `[]string` | `["kube-system"]` | Namespaces exempt from enforcement. |
| `exemptions.usernames` | `[]string` | `[]` | Usernames exempt from enforcement. |
| `exemptions.runtimeClassNames` | `[]string` | `[]` | RuntimeClassNames exempt from enforcement. |

Version is always set to `latest` and is not configurable.

## Examples

### Enforce restricted Pod Security Standard

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
          podSecurityAdmission:
            enforce: restricted
            audit: restricted
            warn: restricted
```

Applying this configuration will result in the following being applied to the
`KubeadmControlPlaneTemplate`:

- A `PodSecurityConfiguration` file at `/etc/kubernetes/pod-security-admission.yaml`
- An `AdmissionConfiguration` file at `/etc/kubernetes/admission.yaml` referencing the plugin
- The `--admission-control-config-file` and `--enable-admission-plugins` API server extra args
- Volume mounts for both configuration files

### Audit and warn only (no enforcement)

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
          podSecurityAdmission:
            enforce: privileged
            audit: restricted
            warn: restricted
```

### Custom exemptions

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
          podSecurityAdmission:
            enforce: restricted
            audit: restricted
            warn: restricted
            exemptions:
              namespaces:
                - kube-system
                - my-privileged-namespace
              usernames:
                - system:serviceaccount:kube-system:some-sa
              runtimeClassNames:
                - kata
```
