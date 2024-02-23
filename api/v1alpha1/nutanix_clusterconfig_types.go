// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type NutanixSpec struct {
	ControlPlaneEndpoint *NutanixControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty"`
}

func (NutanixSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"controlPlaneEndpoint": NutanixControlPlaneEndpointSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type NutanixControlPlaneEndpointSpec struct {
	Host string `json:"host,omitempty"`
	Port int32  `json:"port,omitempty"`
}

func (NutanixControlPlaneEndpointSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix control-plane endpoint configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"host": {
					Description: "host ip/fqdn for control plane api server",
					Type:        "string",
				},
				"port": {
					Description: "port for control plane api server",
					Type:        "integer",
				},
			},
		},
	}
}
