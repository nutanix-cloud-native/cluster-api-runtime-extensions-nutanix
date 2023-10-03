// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type DockerSpec struct {
	DockerWorkerSpec `json:",inline"`
}

func (DockerSpec) VariableSchema() clusterv1.VariableSchema {
	schema := clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Docker cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"customImage": OCIImage("").VariableSchema().OpenAPIV3Schema,
			},
		},
	}

	maps.Copy(
		schema.OpenAPIV3Schema.Properties,
		DockerWorkerSpecProperties,
	)

	return schema
}
