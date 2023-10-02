// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/variables"
)

//+kubebuilder:object:root=true

// AWSClusterConfig is the Schema for the awsclusterconfigs API.
type AWSClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AWSClusterConfigSpec `json:"spec,omitempty"`
}

// AWSClusterConfigSpec defines the desired state of AWSClusterConfig.
type AWSClusterConfigSpec struct {
	// +optional
	AWS *AWSSpec `json:"aws,omitempty"`

	GenericClusterConfig `json:",inline"`
}

type AWSSpec struct {
	// +optional
	Region *Region `json:"region,omitempty"`
}

func (AWSClusterConfigSpec) VariableSchema() clusterv1.VariableSchema {
	clusterConfigProps := GenericClusterConfig{}.VariableSchema().OpenAPIV3Schema.Properties

	maps.Copy(
		clusterConfigProps,
		map[string]clusterv1.JSONSchemaProps{
			"aws": AWSSpec{}.VariableSchema().OpenAPIV3Schema,
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

func (AWSSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"region": Region("").VariableSchema().OpenAPIV3Schema,
			},
			Required: []string{"region"},
		},
	}
}

type Region string

func (Region) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Default:     variables.MustMarshal("us-west-2"),
			Description: "AWS region to create cluster in",
		},
	}
}

func init() {
	SchemeBuilder.Register(&AWSClusterConfig{})
}
