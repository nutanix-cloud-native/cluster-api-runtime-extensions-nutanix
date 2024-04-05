// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	AWSControlPlaneInstanceType InstanceType = "m5.xlarge"
	AWSWorkerInstanceType       InstanceType = "m5.2xlarge"
)

type AWSNodeSpec struct {
	// +optional
	IAMInstanceProfile *IAMInstanceProfile `json:"iamInstanceProfile,omitempty"`

	// +optional
	InstanceType *InstanceType `json:"instanceType,omitempty"`

	// AMI or AMI Lookup arguments for machine image of a AWS machine.
	// If both AMI ID and AMI lookup arguments are provided then AMI ID takes precedence
	//+optional
	AMISpec *AMISpec `json:"ami,omitempty"`

	//+optional
	AdditionalSecurityGroups AdditionalSecurityGroup `json:"additionalSecurityGroups,omitempty"`
}

func NewAWSControlPlaneNodeSpec() *AWSNodeSpec {
	return &AWSNodeSpec{
		InstanceType: ptr.To(AWSControlPlaneInstanceType),
	}
}

func NewAWSWorkerNodeSpec() *AWSNodeSpec {
	return &AWSNodeSpec{
		InstanceType: ptr.To(AWSWorkerInstanceType),
	}
}

type AdditionalSecurityGroup []SecurityGroup

type SecurityGroup struct {
	// ID is the id of the security group
	// +optional
	ID *string `json:"id,omitempty"`
}

func (AdditionalSecurityGroup) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "array",
			Items: &clusterv1.JSONSchemaProps{
				Type: "object",
				Properties: map[string]clusterv1.JSONSchemaProps{
					"id": {
						Type:        "string",
						Description: "Security group ID to add for the cluster Machines",
					},
				},
			},
		},
	}
}

func (a AWSNodeSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS Node configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"iamInstanceProfile":       IAMInstanceProfile("").VariableSchema().OpenAPIV3Schema,
				"instanceType":             a.InstanceType.VariableSchema().OpenAPIV3Schema,
				"ami":                      AMISpec{}.VariableSchema().OpenAPIV3Schema,
				"additionalSecurityGroups": AdditionalSecurityGroup{}.VariableSchema().OpenAPIV3Schema,
			},
			Required: []string{"instanceType"},
		},
	}
}

type IAMInstanceProfile string

func (IAMInstanceProfile) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Description: "The IAM instance profile to use for the cluster Machines",
		},
	}
}

type InstanceType string

func (i InstanceType) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Description: "The AWS instance type to use for the cluster Machines",
			Default:     variables.MustMarshal(string(i)),
		},
	}
}

type AMISpec struct {
	// ID is an explicit AMI to use.
	// +optional
	ID string `json:"id,omitempty"`

	// Lookup is the lookup arguments for the AMI.
	// +optional
	Lookup *AMILookup `json:"lookup,omitempty"`
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
				"lookup": AMILookup{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type AMILookup struct {
	// Format is the AMI naming format
	// +optional
	Format string `json:"format,omitempty"`

	// Org is the AWS Organization ID to use for image lookup
	// +optional
	Org string `json:"org,omitempty"`

	// BaseOS is the name of the base os for image lookup
	// +optional
	BaseOS string `json:"baseOS,omitempty"`
}

func (AMILookup) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:    "object",
			Default: &v1.JSON{},
			Properties: map[string]clusterv1.JSONSchemaProps{
				"format": {
					Type: "string",
					Description: "AMI naming format. Supports substitutions for {{.BaseOS}} and {{.K8sVersion}} with the" +
						"base OS and kubernetes version. example: capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*",
				},
				"org": {
					Type:        "string",
					Description: "The AWS Organization ID to use for image lookup",
				},
				"baseOS": {
					Type:        "string",
					Description: "The name of the base os for image lookup",
				},
			},
		},
	}
}
