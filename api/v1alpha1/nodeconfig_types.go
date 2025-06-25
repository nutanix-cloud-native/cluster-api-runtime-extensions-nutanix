// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	_ "embed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

var (
	//go:embed crds/caren.nutanix.com_dockerworkernodeconfigs.yaml
	dockerNodeConfigCRDDefinition []byte
	//go:embed crds/caren.nutanix.com_awsworkernodeconfigs.yaml
	awsNodeConfigCRDDefinition []byte
	//go:embed crds/caren.nutanix.com_nutanixworkernodeconfigs.yaml
	nutanixNodeConfigCRDDefinition []byte

	dockerNodeConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		dockerNodeConfigCRDDefinition,
	)
	awsWorkerNodeConfigVariableSchema = variables.MustSchemaFromCRDYAML(awsNodeConfigCRDDefinition)
	nutanixNodeConfigVariableSchema   = variables.MustSchemaFromCRDYAML(
		nutanixNodeConfigCRDDefinition,
	)
)

// +kubebuilder:object:root=true

// AWSWorkerNodeConfig is the Schema for the awsnodeconfigs API.
type AWSWorkerNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec AWSWorkerNodeConfigSpec `json:"spec,omitempty"`
}

func (s AWSWorkerNodeConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return awsWorkerNodeConfigVariableSchema
}

// AWSWorkerNodeConfigSpec defines the desired state of AWSNodeConfig.
// Place any configuration that can be applied to individual Nodes here.
// Otherwise, it should go into the ClusterConfigSpec.
type AWSWorkerNodeConfigSpec struct {
	// +kubebuilder:validation:Optional
	AWS *AWSWorkerNodeSpec `json:"aws,omitempty"`

	GenericNodeSpec `json:",inline"`
}

// +kubebuilder:object:root=true

// DockerWorkerNodeConfig is the Schema for the dockerworkernodeconfigs API.
type DockerWorkerNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec DockerControlPlaneSpec `json:"spec,omitempty"`
}

func (s DockerWorkerNodeConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return dockerNodeConfigVariableSchema
}

// DockerWorkerNodeConfigSpec defines the desired state of DockerNodeConfig.
// Place any configuration that can be applied to individual Nodes here.
// Otherwise, it should go into the ClusterConfigSpec.
type DockerWorkerNodeConfigSpec struct {
	// +kubebuilder:validation:Optional
	Docker *DockerNodeSpec `json:"docker,omitempty"`

	GenericNodeSpec `json:",inline"`
}

// +kubebuilder:object:root=true

// NutanixWorkerNodeConfig is the Schema for the nutanixworkernodeconfigs API.
type NutanixWorkerNodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec NutanixWorkerNodeConfigSpec `json:"spec,omitempty"`
}

func (s NutanixWorkerNodeConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return nutanixNodeConfigVariableSchema
}

// NutanixWorkerNodeConfigSpec defines the desired state of NutanixWorkerNodeSpec.
type NutanixWorkerNodeConfigSpec struct {
	// +kubebuilder:validation:Optional
	Nutanix *NutanixWorkerNodeSpec `json:"nutanix,omitempty"`

	GenericNodeSpec `json:",inline"`
}

type GenericNodeSpec struct {
	// Taints specifies the taints the Node API object should be registered with.
	// +kubebuilder:validation:Optional
	Taints []Taint `json:"taints,omitempty"`

	// NodeRegistration holds fields that relate to registering the new control-plane node to the cluster.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	NodeRegistration *NodeRegistrationOptions `json:"nodeRegistration,omitempty"`
}

// The node this Taint is attached to has the "effect" on
// any pod that does not tolerate the Taint.
type Taint struct {
	// The taint key to be applied to a node.
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// The taint value corresponding to the taint key.
	// +kubebuilder:validation:Optional
	Value string `json:"value,omitempty"`

	// The effect of the taint on pods that do not tolerate the taint.
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=NoSchedule
	// +kubebuilder:validation:Enum:=NoSchedule;PreferNoSchedule;NoExecute
	Effect TaintEffect `json:"effect"`
}

type TaintEffect string

const (
	// Do not allow new pods to schedule onto the node unless they tolerate the taint,
	// but allow all pods submitted to Kubelet without going through the scheduler
	// to start, and allow all already-running pods to continue running.
	// Enforced by the scheduler.
	TaintEffectNoSchedule TaintEffect = "NoSchedule"

	// Like TaintEffectNoSchedule, but the scheduler tries not to schedule
	// new pods onto the node, rather than prohibiting new pods from scheduling
	// onto the node entirely. Enforced by the scheduler.
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"

	// Evict any already-running pods that do not tolerate the taint.
	// Currently enforced by NodeController.
	TaintEffectNoExecute TaintEffect = "NoExecute"
)

// NodeRegistrationOptions holds fields that relate to registering a new control-plane or node to the cluster,
// either via "kubeadm init" or "kubeadm join".
type NodeRegistrationOptions struct {
	// IgnorePreflightErrors specifies a slice of pre-flight errors to be ignored by kubeadm
	// when the current node is registered.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={"SystemVerification"}
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:items:MinLength=1
	// +kubebuilder:validation:items:MaxLength=512
	IgnorePreflightErrors []string `json:"ignorePreflightErrors,omitempty"`
}

//nolint:gochecknoinits // Idiomatic to use init functions to register APIs with scheme.
func init() {
	objectTypes = append(objectTypes,
		&AWSWorkerNodeConfig{},
		&DockerWorkerNodeConfig{},
		&NutanixWorkerNodeConfig{},
	)
}
