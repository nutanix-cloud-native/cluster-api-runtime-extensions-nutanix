// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capav1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"github.com/d2iq-labs/capi-runtime-extensions/api/variables"
)

type AWSSpec struct {
	// AWS region to create cluster in.
	// +optional
	Region *Region `json:"region,omitempty"`
	// +optional
	Network *AWSNetwork `json:"network,omitempty"`
	// +optional
	ControlPlaneLoadBalancer *AWSLoadBalancerSpec `json:"controlPlaneLoadBalancer,omitempty"`
}

func (AWSSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"region":                   Region("").VariableSchema().OpenAPIV3Schema,
				"network":                  AWSNetwork{}.VariableSchema().OpenAPIV3Schema,
				"controlPlaneLoadBalancer": AWSLoadBalancerSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type Region string

func (Region) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS region to create cluster in",
			Type:        "string",
		},
	}
}

type AWSNetwork struct {
	// +optional
	VPC *VPC `json:"vpc,omitempty"`

	// +optional
	Subnets Subnets `json:"subnets,omitempty"`
}

func (AWSNetwork) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS network configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"vpc":     VPC{}.VariableSchema().OpenAPIV3Schema,
				"subnets": Subnets{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type VPC struct {
	// ID is the vpc-id of the VPC this provider should use to create resources.
	ID string `json:"id,omitempty"`
}

func (VPC) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS VPC configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"id": {
					Description: "Existing VPC ID to use for the cluster",
					Type:        "string",
				},
			},
		},
	}
}

type Subnets []SubnetSpec

func (Subnets) VariableSchema() clusterv1.VariableSchema {
	resourceSchema := SubnetSpec{}.VariableSchema().OpenAPIV3Schema

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS Subnet configurations",
			Type:        "array",
			Items:       &resourceSchema,
		},
	}
}

// SubnetSpec configures an AWS Subnet.
type SubnetSpec struct {
	// ID defines a unique identifier to reference this resource.
	ID string `json:"id"`
}

func (SubnetSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "An AWS Subnet configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"id": {
					Description: "Existing Subnet ID to use for the cluster",
					Type:        "string",
				},
			},
		},
	}
}

// AWSLoadBalancerSpec configures an AWS control-plane LoadBalancer.
type AWSLoadBalancerSpec struct {
	// Scheme sets the scheme of the load balancer (defaults to internet-facing)
	// +kubebuilder:default=internet-facing
	// +kubebuilder:validation:Enum=internet-facing;internal
	// +optional
	Scheme *capav1.ELBScheme `json:"scheme,omitempty"`
}

func (AWSLoadBalancerSpec) VariableSchema() clusterv1.VariableSchema {
	supportedScheme := []capav1.ELBScheme{capav1.ELBSchemeInternetFacing, capav1.ELBSchemeInternal}

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS control-plane LoadBalancer configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"scheme": {
					Description: "Scheme sets the scheme of the load balancer (defaults to internet-facing)",
					Type:        "string",
					Enum:        variables.MustMarshalValuesToEnumJSON(supportedScheme...),
				},
			},
		},
	}
}
