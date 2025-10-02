// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
)

type EKSSpec struct {
	// AdditionalTags is an optional set of tags to add to an instance,
	// in addition to the ones added by default by the AWS provider.
	// +optional
	AdditionalTags capav1.Tags `json:"additionalTags,omitempty"`

	// IdentityRef is a reference to an identity to be used when reconciling the managed control plane.
	// If no identity is specified, the default identity for this controller will be used.
	// +kubebuilder:validation:Optional
	IdentityRef *capav1.AWSIdentityReference `json:"identityRef,omitempty"`

	// AWS region to create cluster in.
	// +kubebuilder:validation:Optional
	Region *Region `json:"region,omitempty"`

	// AWS network configuration.
	// +kubebuilder:validation:Optional
	Network *AWSNetwork `json:"network,omitempty"`
}
