// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// VSphereSpec defines the desired state of VSphereCluster.
type VSphereSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// host can be either DNS name or ip address
	// +kubebuilder:validation:Required
	ControlPlaneEndpoint ControlPlaneEndpointSpec `json:"controlPlaneEndpoint"`
}
