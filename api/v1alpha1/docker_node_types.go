// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type DockerNodeSpec struct {
	// Custom OCI image for control plane and worker Nodes.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^((?:[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*|\[(?:[a-fA-F0-9:]+)\])(:[0-9]+)?/)?[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*)*(:[\w][\w.-]{0,127})?(@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][0-9A-Fa-f]{32,})?$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	CustomImage string `json:"customImage,omitempty"`
}
