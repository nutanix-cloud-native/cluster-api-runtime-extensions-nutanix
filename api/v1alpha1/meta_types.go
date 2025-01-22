// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// TypedLocalObjectReference contains enough information to let you locate the
// typed referenced object inside the same namespace.
// This is redacted from the upstream https://pkg.go.dev/k8s.io/api/core/v1#TypedLocalObjectReference
type TypedLocalObjectReference struct {
	// Kind is the type of resource being referenced, valid values are ('Secret', 'ConfigMap').
	// +kubebuilder:validation:Enum=Secret;ConfigMap
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name is the name of resource being referenced.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}
