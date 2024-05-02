// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create. This is a copy of customizable fields from metav1.ObjectMeta.
//
// For more details on why this is included instead of using metav1.ObjectMeta directly, see
// https://github.com/kubernetes-sigs/cluster-api/blob/v1.3.3/api/v1beta1/common_types.go#L175-L195.
type ObjectMeta struct {
	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +kubebuilder:validation:Optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +kubebuilder:validation:Optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ControlPlaneEndpointSpec struct {
	// The hostname on which the API server is serving.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// The port on which the API server is serving.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// Configuration for the virtual IP provider.
	// +kubebuilder:validation:Optional
	VirtualIPSpec *ControlPlaneVirtualIPSpec `json:"virtualIP,omitempty"`
}

type ControlPlaneVirtualIPSpec struct {
	// Virtual IP provider to deploy.
	// +kubebuilder:validation:Enum=KubeVIP
	// +kubebuilder:default=KubeVIP
	// +kubebuilder:validation:Optional
	Provider string `json:"provider,omitempty"`
}

// LocalObjectReference contains enough information to let you locate the
// referenced object inside the same namespace.
type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}
