// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

type AWSNodeSpec struct {
	// +optional
	IAMInstanceProfile *IAMInstanceProfile `json:"iamInstanceProfile,omitempty"`

	// +optional
	InstanceType *InstanceType `json:"instanceType,omitempty"`
}

func (AWSNodeSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS Node configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"iamInstanceProfile": IAMInstanceProfile("").VariableSchema().OpenAPIV3Schema,
				"instanceType":       InstanceType("").VariableSchema().OpenAPIV3Schema,
			},
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

func (InstanceType) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Description: "The AWS instance type to use for the cluster Machines",
		},
	}
}
