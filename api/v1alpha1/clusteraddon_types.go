// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterAddonSpec defines the desired state of ClusterAddon.
type ClusterAddonSpec struct {
	// ImplementationRef is a required reference to a custom resource
	// offered by an cluster addon provider.
	ImplementationRef corev1.TypedLocalObjectReference `json:"implementationRef"`

	// Version defines the desired addon version.
	// This field is meant to be optionally used by addon providers.
	// +optional
	Version *string `json:"version,omitempty"`
}

// ClusterAddonStatus defines the observed state of ClusterAddon.
type ClusterAddonStatus struct{}

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
