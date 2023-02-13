// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterAddonSetSpec defines the desired state of ClusterAddonSet.
type ClusterAddonSetSpec struct {
	// ClusterSelector selects Clusters in the same namespace with a label that matches the specified label selector.
	ClusterSelector metav1.LabelSelector `json:"clusterSelector"`

	Template ClusterAddonSetTemplateSpec `json:"template,omitempty"`
}

// ClusterAddonSetTemplateSpec describes the data needed to create a ClusterAddon from a template.
type ClusterAddonSetTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the cluster addon.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec ClusterAddonSpec `json:"spec,omitempty"`
}

// ClusterAddonSetStatus defines the observed state of ClusterAddon.
type ClusterAddonSetStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterAddonSet is the Schema for the clusteraddons API.
type ClusterAddonSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAddonSetSpec   `json:"spec,omitempty"`
	Status ClusterAddonSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterAddonSetList contains a list of ClusterAddon.
type ClusterAddonSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAddonSet `json:"items"`
}

//nolint:gochecknoinits // Idiomatic way to register k8s API kinds.
func init() {
	SchemeBuilder.Register(&ClusterAddonSet{}, &ClusterAddonSetList{})
}
