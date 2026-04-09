<!--
 Copyright 2024 Nutanix. All rights reserved.
 SPDX-License-Identifier: Apache-2.0
 -->

# NCN-101538: Pod Security Admission Configuration via CAREN Variables

## Overview

Add support for configuring the Kubernetes [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/)
(PSA) plugin via CAREN cluster configuration variables. This enables users to set cluster-wide PSA defaults
(enforce, audit, warn levels) and exemptions at cluster creation time, without manually crafting kubeadm
configuration or kustomize patches.

## Scope

- **In scope:** Cluster-wide PSA defaults via the `PodSecurity` admission plugin configuration file,
  configurable through a new `podSecurityAdmission` field on `KubeadmClusterConfigSpec`.
- **Out of scope:** Per-namespace PSA labels, EKS support (EKS does not expose the admission config file),
  refactoring existing `EventRateLimit` CIS patches to use the new shared package.

## Providers

Kubeadm-based providers only: **AWS, Docker, Nutanix**. Not EKS (managed control plane — no access to
`--admission-control-config-file`).

## API Types

### New types in `api/v1alpha1/`

```go
// PodSecurityStandard defines the Pod Security Standard levels.
// +kubebuilder:validation:Enum=privileged;baseline;restricted
type PodSecurityStandard string

const (
    PodSecurityStandardPrivileged PodSecurityStandard = "privileged"
    PodSecurityStandardBaseline   PodSecurityStandard = "baseline"
    PodSecurityStandardRestricted PodSecurityStandard = "restricted"
)

// PodSecurityAdmission configures the PodSecurity admission plugin with cluster-wide defaults.
// When not specified, no PodSecurity admission configuration is applied (no-op for existing clusters).
type PodSecurityAdmission struct {
    // Enforce sets the level for the enforce mode.
    // Pods that violate this level will be rejected.
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=privileged
    // +kubebuilder:validation:Enum=privileged;baseline;restricted
    Enforce PodSecurityStandard `json:"enforce,omitempty"`

    // Audit sets the level for the audit mode.
    // Violations are recorded in the API server audit log.
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=privileged
    // +kubebuilder:validation:Enum=privileged;baseline;restricted
    Audit PodSecurityStandard `json:"audit,omitempty"`

    // Warn sets the level for the warn mode.
    // Violations trigger a user-facing warning.
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=privileged
    // +kubebuilder:validation:Enum=privileged;baseline;restricted
    Warn PodSecurityStandard `json:"warn,omitempty"`

    // Exemptions defines the exemptions from pod security enforcement.
    // +kubebuilder:validation:Optional
    Exemptions PodSecurityExemptions `json:"exemptions,omitempty"`
}

// PodSecurityExemptions defines resources exempt from pod security enforcement.
type PodSecurityExemptions struct {
    // Namespaces that are exempt from pod security enforcement.
    // +kubebuilder:validation:Optional
    // +kubebuilder:default={"kube-system"}
    // +kubebuilder:validation:MaxItems=64
    // +kubebuilder:validation:items:MinLength=1
    // +kubebuilder:validation:items:MaxLength=63
    Namespaces []string `json:"namespaces,omitempty"`

    // Usernames that are exempt from pod security enforcement.
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:MaxItems=64
    // +kubebuilder:validation:items:MinLength=1
    // +kubebuilder:validation:items:MaxLength=256
    Usernames []string `json:"usernames,omitempty"`

    // RuntimeClassNames that are exempt from pod security enforcement.
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:MaxItems=64
    // +kubebuilder:validation:items:MinLength=1
    // +kubebuilder:validation:items:MaxLength=63
    RuntimeClassNames []string `json:"runtimeClassNames,omitempty"`
}
```

### Modified type

`KubeadmClusterConfigSpec` gains a new optional pointer field:

```go
type KubeadmClusterConfigSpec struct {
    // ... existing fields ...

    // PodSecurityAdmission configures the PodSecurity admission plugin
    // with cluster-wide defaults.
    // +kubebuilder:validation:Optional
    PodSecurityAdmission *PodSecurityAdmission `json:"podSecurityAdmission,omitempty"`
}
```

### Defaults behavior

- `podSecurityAdmission` is a pointer (`*PodSecurityAdmission`). When omitted, it is `nil` and the handler
  is a complete no-op — no patches, no rollout for existing clusters.
- When explicitly set (even as `podSecurityAdmission: {}`), kubebuilder defaults apply:
  - `enforce`, `audit`, `warn` all default to `privileged`
  - `exemptions.namespaces` defaults to `["kube-system"]`
- Version is always `latest` — not user-configurable.

### User-facing YAML example

```yaml
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: my-cluster
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
                - my-privileged-ns
              usernames:
                - system:serviceaccount:kube-system:some-sa
```

## Architecture

### Shared package: `pkg/handlers/generic/mutation/kubeadm/admissionconfiguration/`

A reusable package for any handler that needs to add an admission plugin to the API server's
`AdmissionConfiguration`. Handles all coordination with existing admission configuration.

**`AddPlugin(kcpTemplate, plugin) error`**

Logic:
1. Check `apiServer.ExtraArgs` for `admission-control-config-file`.
2. If the arg exists:
   - Read the path value from the arg.
   - Find the matching file in `KubeadmConfigSpec.Files` by that path.
   - If found: parse the existing `AdmissionConfiguration` YAML, append the new plugin entry,
     update the file content in-place.
   - If not found: create a new `AdmissionConfiguration` at that path with the plugin entry,
     add the file and volume mount.
3. If the arg doesn't exist:
   - Use default path `/etc/kubernetes/admission.yaml`.
   - Create a new `AdmissionConfiguration` with the plugin entry.
   - Set the `admission-control-config-file` extra arg.
   - Add the file and volume mount.
4. Add the plugin's own config file and volume mount.
5. Append the plugin name to `enable-admission-plugins` (deduplicating).

```go
type Plugin struct {
    Name              string // e.g. "PodSecurity"
    ConfigFilePath    string // e.g. "/etc/kubernetes/pod-security-admission.yaml"
    ConfigFileContent string // serialized plugin config
}
```

### PSA handler: `pkg/handlers/generic/mutation/kubeadm/podsecurityadmission/`

Thin handler that:
1. Reads `clusterConfig.podSecurityAdmission` variable.
2. If nil → return nil (no-op).
3. Generates a `PodSecurityConfiguration` YAML (apiVersion `pod-security.admission.config.k8s.io/v1`).
4. Calls `admissionconfiguration.AddPlugin()` with the generated config.

Generated `PodSecurityConfiguration` example:

```yaml
apiVersion: pod-security.admission.config.k8s.io/v1
kind: PodSecurityConfiguration
defaults:
  enforce: "restricted"
  enforce-version: "latest"
  audit: "privileged"
  audit-version: "latest"
  warn: "privileged"
  warn-version: "latest"
exemptions:
  namespaces:
    - kube-system
```

### Handler registration

Added to `MetaMutators()` in `pkg/handlers/generic/mutation/handlers.go`, before
`containerdapplypatchesandrestart` (which must remain last) and before `sortextraargs`.

### Handler version safety

This handler produces new patches **only when `podSecurityAdmission` is explicitly set** in the cluster
variables. Existing clusters without this field get zero patches. No handler version bump is required.

## Testing

### Shared package tests (`admissionconfiguration/`)

- No existing admission config → creates new `AdmissionConfiguration`, sets extra arg, adds file + volume mount.
- Existing admission config file and extra arg → parses existing config, appends new plugin, preserves
  existing plugins.
- Existing extra arg but no matching file → creates file at the path from the extra arg.
- Plugin already present → idempotent, does not duplicate.
- `enable-admission-plugins` deduplication → appends plugin name only if not already listed.

### PSA handler tests (`podsecurityadmission/inject_test.go`)

Using Ginkgo / `capitest.AssertGeneratePatches` pattern:
- Variable not set → no patches (no-op).
- All defaults (`podSecurityAdmission: {}`) → config with `privileged`/`privileged`/`privileged`,
  `kube-system` exemption.
- Enforce restricted → correct `PodSecurityConfiguration` with `enforce: restricted`, others `privileged`.
- All modes set → all three modes reflected in config.
- Custom exemptions → namespaces, usernames, runtimeClassNames all appear in config.
- Existing admission config file → PSA plugin appended, existing plugins preserved.

## Documentation

New page at `docs/content/customization/generic/pod-security-admission.md`:
- What PSA is and what this feature configures.
- Example YAML showing the variable in a Cluster resource.
- Table of fields with defaults.
- Note that this is kubeadm-only (not EKS).
- Note on the `kube-system` default exemption.
