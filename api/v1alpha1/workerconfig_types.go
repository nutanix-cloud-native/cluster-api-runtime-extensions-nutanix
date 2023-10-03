// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

//+kubebuilder:object:root=true

// WorkerConfig is the Schema for the workerconfigs API.
type WorkerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+optional
	Spec WorkerConfigSpec `json:"spec,omitempty"`
}

// WorkerConfigSpec defines the desired state of WorkerConfig.
// Place any configuration that can be applied to individual worker Nodes here.
// Otherwise, it should go into the ClusterConfigSpec.
type WorkerConfigSpec struct {
	// +optional
	AWS *AWSWorkerSpec `json:"aws,omitempty"`
	// +optional
	Docker *DockerWorkerSpec `json:"docker,omitempty"`
}

func (s WorkerConfigSpec) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	workerConfigProps := GenericWorkerConfig{}.VariableSchema()

	switch {
	case s.AWS != nil:
		maps.Copy(
			workerConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"aws": AWSWorkerSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)

		workerConfigProps.OpenAPIV3Schema.Required = append(
			workerConfigProps.OpenAPIV3Schema.Required,
			"aws",
		)
	case s.Docker != nil:
		maps.Copy(
			workerConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"docker": DockerWorkerSpec{}.VariableSchema().OpenAPIV3Schema,
			},
		)

		workerConfigProps.OpenAPIV3Schema.Required = append(
			workerConfigProps.OpenAPIV3Schema.Required,
			"docker",
		)
	}

	return workerConfigProps
}

type GenericWorkerConfig struct {
}

func (GenericWorkerConfig) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Worker Node configuration",
			Type:        "object",
			Properties:  map[string]clusterv1.JSONSchemaProps{},
		},
	}
}

func init() {
	SchemeBuilder.Register(&WorkerConfig{})
}
