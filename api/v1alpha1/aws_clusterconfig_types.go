// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
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
	Region *Region `json:"region,omitempty"`
}

func (AWSClusterConfigSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "AWS Cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"region": Region("").VariableSchema().OpenAPIV3Schema,
			},
			Required: []string{"region"},
		},
	}
}

type Region string

const (
	RegionAFSouth1     Region = "af-south-1"
	RegionAPEast1      Region = "ap-east-1"
	RegionAPNorthEast1 Region = "ap-northeast-1"
	RegionAPNorthEast2 Region = "ap-northeast-2"
	RegionAPNorthEast3 Region = "ap-northeast-3"
	RegionAPSouth1     Region = "ap-south-1"
	RegionAPSouth2     Region = "ap-south-2"
	RegionAPSouthEast1 Region = "ap-southeast-1"
	RegionAPSouthEast2 Region = "ap-southeast-2"
	RegionAPSouthEast3 Region = "ap-southeast-3"
	RegionAPSouthEast4 Region = "ap-southeast-4"
	RegionCACentral1   Region = "ca-central-1"
	RegionEUCentral1   Region = "eu-central-1"
	RegionEUCentral2   Region = "eu-central-2"
	RegionEUNorth1     Region = "eu-north-1"
	RegionEUSouth1     Region = "eu-south-1"
	RegionEUSouth2     Region = "eu-south-2"
	RegionEUWest1      Region = "eu-west-1"
	RegionEUWest2      Region = "eu-west-2"
	RegionEUWest3      Region = "eu-west-3"
	RegionILCentral1   Region = "il-central-1"
	RegionMECentral1   Region = "me-central-1"
	RegionMESouth1     Region = "me-south-1"
	RegionSAEast1      Region = "sa-east-1"
	RegionUSEast1      Region = "us-east-1"
	RegionUSEast2      Region = "us-east-2"
	RegionUSWest1      Region = "us-west-1"
	RegionUSWest2      Region = "us-west-2"
)

func (Region) VariableSchema() clusterv1.VariableSchema {
	allRegions := []Region{
		RegionAFSouth1,
		RegionAPEast1,
		RegionAPNorthEast1,
		RegionAPNorthEast2,
		RegionAPNorthEast3,
		RegionAPSouth1,
		RegionAPSouth2,
		RegionAPSouthEast1,
		RegionAPSouthEast2,
		RegionAPSouthEast3,
		RegionAPSouthEast4,
		RegionCACentral1,
		RegionEUCentral1,
		RegionEUCentral2,
		RegionEUNorth1,
		RegionEUSouth1,
		RegionEUSouth2,
		RegionEUWest1,
		RegionEUWest2,
		RegionEUWest3,
		RegionILCentral1,
		RegionMECentral1,
		RegionMESouth1,
		RegionSAEast1,
		RegionUSEast1,
		RegionUSEast2,
		RegionUSWest1,
		RegionUSWest2,
	}

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Enum:        variables.MustMarshalValuesToEnumJSON(allRegions...),
			Default:     variables.MustMarshal(RegionUSWest2),
			Description: "AWS region to create cluster in",
		},
	}
}

func init() {
	SchemeBuilder.Register(&AWSClusterConfig{})
}
