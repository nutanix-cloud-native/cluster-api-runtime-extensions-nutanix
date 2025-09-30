// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
)

type AWSSpec struct {
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

	// +kubebuilder:validation:Optional
	ControlPlaneLoadBalancer *AWSLoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`
}

// +kubebuilder:validation:MinLength=4
// +kubebuilder:validation:MaxLength=16
type Region string

type AWSNetwork struct {
	// +kubebuilder:validation:Optional
	VPC *VPC `json:"vpc,omitempty"`

	// AWS Subnet configuration.
	// +kubebuilder:validation:Optional
	Subnets Subnets `json:"subnets,omitempty"`
}

type VPC struct {
	// Existing VPC ID to use for the cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=`^vpc-[0-9a-f]{8}(?:[0-9a-f]{9})?$`
	ID string `json:"id"`
}

// +kubebuilder:validation:MaxItems=10
type Subnets []SubnetSpec

// SubnetSpec configures an AWS Subnet.
type SubnetSpec struct {
	// Existing Subnet ID to use for the cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=`^subnet-[0-9a-f]{8}(?:[0-9a-f]{9})?$`
	ID string `json:"id"`
}

// AWSLoadBalancerSpec configures an AWS control-plane LoadBalancer.
type AWSLoadBalancerSpec struct {
	// Scheme sets the scheme of the load balancer.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=internet-facing
	// +kubebuilder:validation:Enum=internet-facing;internal
	Scheme *capav1.ELBScheme `json:"scheme,omitempty"`
}
