// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/openapi/patterns"
)

type DockerWorkerSpec struct {
	//+optional
	CustomImage *OCIImage `json:"customImage,omitempty"`
}

var DockerWorkerSpecProperties = map[string]clusterv1.JSONSchemaProps{
	"customImage": OCIImage("").VariableSchema().OpenAPIV3Schema,
}

func (DockerWorkerSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Docker worker configuration",
			Type:        "object",
			Properties:  DockerWorkerSpecProperties,
		},
	}
}

type OCIImage string

func (OCIImage) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Custom OCI image for control plane and worker nodes.",
			Type:        "string",
			Pattern:     patterns.Anchored(patterns.ImageReference),
		},
	}
}
