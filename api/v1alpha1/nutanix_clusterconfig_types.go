// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	PrismCentralPort = 9440
)

// NutanixSpec defines the desired state of NutanixCluster.
type NutanixSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// host can be either DNS name or ip address
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// Nutanix Prism Central endpoint configuration.
	PrismCentralEndpoint NutanixPrismCentralEndpointSpec `json:"prismCentralEndpoint"`
}

func (NutanixSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"controlPlaneEndpoint": ControlPlaneEndpointSpec{}.VariableSchema().OpenAPIV3Schema,
				"prismCentralEndpoint": NutanixPrismCentralEndpointSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type NutanixPrismCentralEndpointSpec struct {
	// host is the DNS name or IP address of the Nutanix Prism Central
	Host string `json:"host"`

	// port is the port number to access the Nutanix Prism Central
	Port int32 `json:"port"`

	// use insecure connection to Prism Central endpoint
	// +optional
	Insecure bool `json:"insecure"`

	// A base64 PEM encoded x509 cert for the RootCA that was used to create
	// the certificate for a Prism Central that uses certificates that were issued by a non-publicly trusted RootCA.
	// The trust bundle is added to the cert pool used to authenticate the TLS connection to the Prism Central.
	// +optional
	AdditionalTrustBundle *string `json:"additionalTrustBundle,omitempty"`

	// A reference to the Secret for credential information for the target Prism Central instance
	Credentials corev1.LocalObjectReference `json:"credentials"`
}

func (NutanixPrismCentralEndpointSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Prism Central endpoint configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"host": {
					Description: "the DNS name or IP address of the Nutanix Prism Central",
					Type:        "string",
					MinLength:   ptr.To[int64](1),
				},
				"port": {
					Description: "The port number to access the Nutanix Prism Central",
					Type:        "integer",
					Default:     variables.MustMarshal(PrismCentralPort),
					Minimum:     ptr.To[int64](1),
					Maximum:     ptr.To[int64](65535),
				},
				"insecure": {
					Description: "Use insecure connection to Prism Central endpoint",
					Type:        "boolean",
				},
				"additionalTrustBundle": {
					Description: "A base64 PEM encoded x509 cert for the RootCA " +
						"that was used to create the certificate for a Prism Central that uses certificates " +
						"that were issued by a non-publicly trusted RootCA." +
						"The trust bundle is added to the cert pool used to authenticate the TLS connection " +
						"to the Prism Central.",
					Type:   "string",
					Format: "byte",
				},
				"credentials": {
					Description: "A reference to the Secret for credential information" +
						"for the target Prism Central instance",
					Type: "object",
					Properties: map[string]clusterv1.JSONSchemaProps{
						"name": {
							Description: "The name of the Secret",
							Type:        "string",
						},
					},
					Required: []string{"name"},
				},
			},
			Required: []string{"host", "port", "credentials"},
		},
	}
}
