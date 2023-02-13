// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterAddonSpec defines the desired state of ClusterAddon.
type ClusterAddonSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ClusterAddon. Edit clusteraddon_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// ClusterAddonStatus defines the observed state of ClusterAddon.
type ClusterAddonStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterAddon is the Schema for the clusteraddons API.
type ClusterAddon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAddonSpec   `json:"spec,omitempty"`
	Status ClusterAddonStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterAddonList contains a list of ClusterAddon.
type ClusterAddonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAddon `json:"items"`
}

//nolint:gochecknoinits // Idiomatic way to register k8s API kinds.
func init() {
	SchemeBuilder.Register(&ClusterAddon{}, &ClusterAddonList{})
}
