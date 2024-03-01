// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type NutanixSpec struct {
	PrismCentralEndpoint *NutanixPrismCentralEndpointSpec `json:"prismCentralEndpoint,omitempty"`
	ControlPlaneEndpoint *NutanixControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty"`
	FailureDomains       []NutanixFailureDomain           `json:"failureDomains,omitempty"`
}

func (NutanixSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"prismCentralEndpoint": NutanixPrismCentralEndpointSpec{}.VariableSchema().OpenAPIV3Schema,
				"controlPlaneEndpoint": NutanixControlPlaneEndpointSpec{}.VariableSchema().OpenAPIV3Schema,
				"failureDomains":       NutanixFailureDomains{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type NutanixPrismCentralEndpointSpec struct {
	Host                  string `json:"host,omitempty"`
	Port                  int32  `json:"port,omitempty"`
	Insecure              bool   `json:"insecure,omitempty"`
	AdditionalTrustBundle string `json:"additionalTrustBundle,omitempty"`
}

func (NutanixPrismCentralEndpointSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Prism Central endpoint configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"host": {
					Description: "host ip/fqdn for Prism Central Server",
					Type:        "string",
				},
				"port": {
					Description: "port for Prism Central Server",
					Type:        "integer",
				},
				"insecure": {
					Description: "Prism Central Certificate checking",
					Type:        "boolean",
				},
				"additionalTrustBundle": {
					Description: "Certificate trust bundle used for Prism Central connection",
					Type:        "string",
				},
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
					Description: "host ip/fqdn for control plane API Server",
					Type:        "string",
				},
				"port": {
					Description: "port for control plane API Server",
					Type:        "integer",
				},
			},
		},
	}
}

type NutanixFailureDomains []NutanixFailureDomain

func (NutanixFailureDomains) VariableSchema() clusterv1.VariableSchema {
	resourceSchema := NutanixFailureDomain{}.VariableSchema().OpenAPIV3Schema

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix failure domains",
			Type:        "array",
			Items:       &resourceSchema,
		},
	}
}

type NutanixFailureDomain struct {
	// name defines the unique name of a failure domain.
	// Name is required and must be at most 64 characters in length.
	// It must consist of only lower case alphanumeric characters and hyphens (-).
	// It must start and end with an alphanumeric character.
	// This value is arbitrary and is used to identify the failure domain within the platform.
	Name string `json:"name"`

	// cluster is to identify the cluster (the Prism Element under management of the Prism Central),
	// in which the Machine's VM will be created. The cluster identifier (uuid or name) can be obtained
	// from the Prism Central console or using the prism_central API.
	Cluster NutanixResourceIdentifier `json:"cluster"`

	// subnets holds a list of identifiers (one or more) of the cluster's network subnets
	// for the Machine's VM to connect to. The subnet identifiers (uuid or name) can be
	// obtained from the Prism Central console or using the prism_central API.
	Subnets []NutanixResourceIdentifier `json:"subnets"`

	// indicates if a failure domain is suited for control plane nodes
	ControlPlane bool `json:"controlPlane,omitempty"`
}

func (NutanixFailureDomain) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Failure Domain",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"name": {
					Description: "name of failure domain",
					Type:        "string",
				},
				"cluster": NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema,
				"subnets": NutanixResourceIdentifiers{}.VariableSchema().OpenAPIV3Schema,
				"controlPlane": {
					Description: "indicates if a failure domain is suited for control plane nodes",
					Type:        "boolean",
				},
			},
		},
	}
}

type NutanixResourceIdentifiers []NutanixResourceIdentifier

func (NutanixResourceIdentifiers) VariableSchema() clusterv1.VariableSchema {
	resourceSchema := NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix resource identifier",
			Type:        "array",
			Items:       &resourceSchema,
		},
	}
}
