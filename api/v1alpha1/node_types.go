// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

//+kubebuilder:object:root=true

// NodeConfig is the Schema for the workerconfigs API.
type NodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+optional
	Spec NodeConfigSpec `json:"spec,omitempty"`
}

// NodeConfigSpec defines the desired state of NodeConfig.
// Place any configuration that can be applied to individual Nodes here.
// Otherwise, it should go into the ClusterConfigSpec.
type NodeConfigSpec struct {
	// +optional
	AWS *AWSNodeSpec `json:"aws,omitempty"`
	// +optional
	Docker *DockerNodeSpec `json:"docker,omitempty"`
	// +optional
	Nutanix *NutanixNodeSpec `json:"nutanix,omitempty"`
}

func (s NodeConfigSpec) VariableSchema() clusterv1.VariableSchema {
	nodeConfigProps := GenericNodeConfig{}.VariableSchema()

	switch {
	case s.AWS != nil:
		maps.Copy(
			nodeConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				AWSVariableName: AWSNodeSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)
	case s.Docker != nil:
		maps.Copy(
			nodeConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"docker": DockerNodeSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)
	case s.Nutanix != nil:
		maps.Copy(
			nodeConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"nutanix": NutanixNodeSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)
	}

	return nodeConfigProps
}

type GenericNodeConfig struct{}

func (GenericNodeConfig) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Node configuration",
			Type:        "object",
			Properties:  map[string]clusterv1.JSONSchemaProps{},
		},
	}
}

func init() {
	SchemeBuilder.Register(&NodeConfig{})
}
