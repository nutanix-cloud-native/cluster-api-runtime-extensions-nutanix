// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type GenericControlPlaneSpec struct {
	// AutoRenewCertificates specifies the configuration for auto-renewing the
	// certificates of the control plane.
	// +kubebuilder:validation:Optional
	AutoRenewCertificates *AutoRenewCertificatesSpec `json:"autoRenewCertificates,omitempty"`
}

type AutoRenewCertificatesSpec struct {
	// DaysBeforeExpiry indicates a rollout needs to be performed if the
	// certificates of the control plane will expire within the specified days.
	// Set to 0 to disable automated certificate renewal.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == 0 || self >= 7",message="Value must be 0 or at least 7"
	DaysBeforeExpiry int32 `json:"daysBeforeExpiry"`
}

// DockerControlPlaneSpec defines the desired state of the control plane for a Docker cluster.
type DockerControlPlaneSpec struct {
	// +kubebuilder:validation:Optional
	Docker *DockerNodeSpec `json:"docker,omitempty"`

	GenericControlPlaneSpec `json:",inline"`

	KubeadmNodeSpec `json:",inline"`
	GenericNodeSpec `json:",inline"`
}

// NutanixControlPlaneSpec defines the desired state of the control plane for a Nutanix cluster.
type NutanixControlPlaneSpec struct {
	// +kubebuilder:validation:Optional
	Nutanix *NutanixControlPlaneNodeSpec `json:"nutanix,omitempty"`

	GenericControlPlaneSpec `json:",inline"`

	KubeadmNodeSpec `json:",inline"`
	GenericNodeSpec `json:",inline"`
}

// AWSControlPlaneSpec defines the desired state of the control plane for an AWS cluster.
type AWSControlPlaneSpec struct {
	// +kubebuilder:validation:Optional
	AWS *AWSControlPlaneNodeSpec `json:"aws,omitempty"`

	GenericControlPlaneSpec `json:",inline"`

	KubeadmNodeSpec `json:",inline"`
	GenericNodeSpec `json:",inline"`
}
