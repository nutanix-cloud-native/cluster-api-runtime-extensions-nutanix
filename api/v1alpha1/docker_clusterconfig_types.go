// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/openapi/patterns"
)

//+kubebuilder:object:root=true

// DockerClusterConfig is the Schema for the dockerclusterconfigs API.
type DockerClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DockerClusterConfigSpec `json:"spec,omitempty"`
}

type DockerSpec struct {
	GenericClusterConfig `json:",inline"`
}

// DockerClusterConfigSpec defines the desired state of DockerClusterConfig.
type DockerClusterConfigSpec struct {
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`

	//+optional
	CustomImage *OCIImage `json:"customImage,omitempty"`
}

func (DockerClusterConfigSpec) VariableSchema() clusterv1.VariableSchema {
	clusterConfigProps := GenericClusterConfig{}.VariableSchema().OpenAPIV3Schema.Properties

	maps.Copy(
		clusterConfigProps,
		map[string]clusterv1.JSONSchemaProps{
			"docker": DockerSpec{}.VariableSchema().OpenAPIV3Schema,
		},
	)

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Cluster configuration",
			Type:        "object",
			Properties:  clusterConfigProps,
		},
	}
}

func (DockerSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Docker cluster configuration",
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
			Description: "Custom OCI image for control plane and worker nodes.",
			Type:        "string",
			Pattern:     patterns.Anchored(patterns.ImageReference),
		},
	}
}

func init() {
	SchemeBuilder.Register(&DockerClusterConfig{})
}
