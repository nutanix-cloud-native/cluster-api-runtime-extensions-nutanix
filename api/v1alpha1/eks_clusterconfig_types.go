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
