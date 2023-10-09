// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type AWSSpec struct {

	// AWS region to create cluster in.
	// +optional
	Region *Region `json:"region,omitempty"`

	// AMI or AMI Lookup arguments for machine image of a AWS machine.
	// If both AMI ID and AMI lookup arguments are provided then AMI ID takes precedence
	//+optional
	AMISpec *AMISpec `json:"amiSpec,omitempty"`
}

func (AWSSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"region": Region("").VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type Region string

func (Region) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Description: "AWS region to create cluster in",
		},
	}
}
