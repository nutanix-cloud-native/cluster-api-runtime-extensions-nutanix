// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

//+kubebuilder:object:root=true

// DockerClusterConfig is the Schema for the dockerclusterconfigs API.
type DockerClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AWSClusterConfigSpec `json:"spec,omitempty"`
}

// DockerClusterConfigSpec defines the desired state of DockerClusterConfig.
type DockerClusterConfigSpec struct {
	GenericClusterConfig `json:",inline"`
}

func (DockerClusterConfigSpec) VariableSchema() clusterv1.VariableSchema {
	clusterConfigProps := GenericClusterConfig{}.VariableSchema().OpenAPIV3Schema.Properties

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Docker cluster configuration",
			Type:        "object",
			Properties:  clusterConfigProps,
		},
	}
}

func init() {
	SchemeBuilder.Register(&DockerClusterConfig{})
}
