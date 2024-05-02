// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type DockerNodeSpec struct {
	// Custom OCI image for control plane and worker Nodes.
	// +kubebuilder:validation:Pattern=`^((?:[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*|\[(?:[a-fA-F0-9:]+)\])(:[0-9]+)?/)?[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*)*(:[\w][\w.-]{0,127})?(@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][0-9A-Fa-f]{32,})?$`
	// +kubebuilder:validation:Optional
	CustomImage *string `json:"customImage,omitempty"`
}
