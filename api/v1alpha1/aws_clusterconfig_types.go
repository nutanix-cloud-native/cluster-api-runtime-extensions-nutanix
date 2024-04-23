// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
)

type AWSSpec struct {
	// AWS region to create cluster in.
	// +optional
	Region *Region `json:"region,omitempty"`
	// AWS network configuration.
	// +optional
	Network *AWSNetwork `json:"network,omitempty"`
	// +optional
	ControlPlaneLoadBalancer *AWSLoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`
}

type Region string

type AWSNetwork struct {
	// +optional
	VPC *VPC `json:"vpc,omitempty"`

	// AWS Subnet configuration.
	// +optional
	Subnets Subnets `json:"subnets,omitempty"`
}

type VPC struct {
	// Existing VPC ID to use for the cluster.
	ID string `json:"id"`
}

type Subnets []SubnetSpec

// SubnetSpec configures an AWS Subnet.
type SubnetSpec struct {
	// Existing Subnet ID to use for the cluster.
	ID string `json:"id"`
}

// AWSLoadBalancerSpec configures an AWS control-plane LoadBalancer.
type AWSLoadBalancerSpec struct {
	// Scheme sets the scheme of the load balancer.
	// +kubebuilder:default=internet-facing
	// +kubebuilder:validation:Enum=internet-facing;internal
	// +optional
	Scheme *capav1.ELBScheme `json:"scheme,omitempty"`
}
