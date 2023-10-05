// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
				"region":  Region("").VariableSchema().OpenAPIV3Schema,
				"amiSpec": AMISpec{}.VariableSchema().OpenAPIV3Schema,
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

type AMISpec struct {
	// ID is an Explicit AMI to use.
	// +optional
	ID string `json:"id,omitempty"`

	// Format is the AMI naming format
	// +optional
	Format string `json:"lookupFormat,omitempty"`

	// Org is the AWS Organization ID to use for image lookup
	// +optional
	Org string `json:"lookupOrg,omitempty"`

	// BaseOS is the name of the base os for image lookup
	// +optional
	BaseOS string `json:"lookupBaseOS,omitempty"`
}

func (AMISpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:    "object",
			Default: &v1.JSON{},
			Description: "AMI or AMI Lookup arguments for machine image of a AWS machine." +
				"If both AMI ID and AMI lookup arguments are provided then AMI ID takes precedence",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"id": {
					Type:        "string",
					Description: "AMI ID is the reference to the AMI from which to create the machine instance.",
				},
				"lookupFormat": {
					Type: "string",
					Description: "AMI naming format. Supports substitutions for {{.BaseOS}} and {{.K8sVersion}} with the" +
						"base OS and kubernetes version. example: capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*",
				},
				"lookupOrg": {
					Type:        "string",
					Description: "The AWS Organization ID to use for image lookup",
				},
				"lookupBaseOS": {
					Type:        "string",
					Description: "The name of the base os for image lookup",
				},
			},
		},
	}
}
