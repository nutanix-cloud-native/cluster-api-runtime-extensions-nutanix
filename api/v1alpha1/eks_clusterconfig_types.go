// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type EKSSpec struct {
	// AWS region to create cluster in.
	// +kubebuilder:validation:Optional
	Region *Region `json:"region,omitempty"`

	// AWS network configuration.
	// +kubebuilder:validation:Optional
	Network *AWSNetwork `json:"network,omitempty"`
}

type EKSKubeProxy struct {
	// Mode specifies the mode for kube-proxy:
	// - disabled means that kube-proxy is disabled.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=disabled
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value cannot be changed after cluster creation"
	Mode KubeProxyMode `json:"mode,omitempty"`
}
