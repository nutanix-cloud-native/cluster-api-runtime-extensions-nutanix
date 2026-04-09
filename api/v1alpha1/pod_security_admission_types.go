// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// PodSecurityStandard defines the Pod Security Standard levels.
// +kubebuilder:validation:Enum=privileged;baseline;restricted
type PodSecurityStandard string

const (
	PodSecurityStandardPrivileged PodSecurityStandard = "privileged"
	PodSecurityStandardBaseline   PodSecurityStandard = "baseline"
	PodSecurityStandardRestricted PodSecurityStandard = "restricted"
)

// PodSecurityAdmission configures the PodSecurity admission plugin with cluster-wide defaults.
// When not specified on KubeadmClusterConfigSpec, no PodSecurity admission configuration is
// applied (no-op for existing clusters).
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
