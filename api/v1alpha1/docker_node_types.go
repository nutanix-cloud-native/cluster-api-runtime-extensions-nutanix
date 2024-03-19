// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/openapi/patterns"
)

type DockerNodeSpec struct {
	// +optional
	CustomImage *OCIImage `json:"customImage,omitempty"`
}

func (DockerNodeSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Docker Node configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"customImage": OCIImage("").VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type OCIImage string

func (OCIImage) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Custom OCI image for control plane and worker Nodes.",
			Type:        "string",
			Pattern:     patterns.Anchored(patterns.ImageReference),
		},
	}
}
