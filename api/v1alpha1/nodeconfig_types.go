// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	_ "embed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

var (
	//go:embed crds/caren.nutanix.com_dockernodeconfigs.yaml
	dockerNodeConfigCRDDefinition []byte
	//go:embed crds/caren.nutanix.com_awsnodeconfigs.yaml
	awsNodeConfigCRDDefinition []byte
	//go:embed crds/caren.nutanix.com_nutanixnodeconfigs.yaml
	nutanixNodeConfigCRDDefinition []byte

	dockerNodeConfigVariableSchema  = variables.MustSchemaFromCRDYAML(dockerNodeConfigCRDDefinition)
	awsNodeConfigVariableSchema     = variables.MustSchemaFromCRDYAML(awsNodeConfigCRDDefinition)
	nutanixNodeConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		nutanixNodeConfigCRDDefinition,
	)
)

//+kubebuilder:object:root=true

// AWSNodeConfig is the Schema for the awsnodeconfigs API.
type AWSNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+optional
	Spec AWSNodeConfigSpec `json:"spec,omitempty"`
}

func (s AWSNodeConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return awsNodeConfigVariableSchema
}

// AWSNodeConfigSpec defines the desired state of AWSNodeConfig.
// Place any configuration that can be applied to individual Nodes here.
// Otherwise, it should go into the ClusterConfigSpec.
type AWSNodeConfigSpec struct {
	// +optional
	AWS *AWSNodeSpec `json:"aws,omitempty"`
}

//+kubebuilder:object:root=true

// DockerNodeConfig is the Schema for the dockernodeconfigs API.
type DockerNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+optional
	Spec DockerNodeConfigSpec `json:"spec,omitempty"`
}

func (s DockerNodeConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return dockerNodeConfigVariableSchema
}

// DockerNodeConfigSpec defines the desired state of DockerNodeSpec.
type DockerNodeConfigSpec struct {
	// +optional
	Docker *DockerNodeSpec `json:"docker,omitempty"`
}

// +kubebuilder:object:root=true

// NutanixNodeConfig is the Schema for the nutanixnodeconfigs API.
type NutanixNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	//+optional
	Spec NutanixNodeConfigSpec `json:"spec,omitempty"`
}

func (s NutanixNodeConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return nutanixNodeConfigVariableSchema
}

// NutanixNodeSpec defines the desired state of NutanixNodeSpec.
type NutanixNodeConfigSpec struct {
	// +optional
	Nutanix *NutanixNodeSpec `json:"nutanix,omitempty"`
}

func init() {
	SchemeBuilder.Register(&AWSNodeConfig{}, &DockerNodeConfig{}, &NutanixNodeConfig{})
}
