// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"

// ClusterConfig is a type to be used internally to simplify the handling of cluster configurations for different
// providers. It is not meant to be used as a CRD.
// By including all the possible configurations for all the providers, we can easily switch between providers in code
// without type assertions/switches and avoids passing around `interface{}` or `any` types.
// Every provider-specific cluster config variable will successfully unmarshal to this type and so it is safe to use
// this internally when a handler provides functionality for multiple providers but exhibits different behaviour per
// provider.
type ClusterConfig struct {
	AWS *carenv1.AWSSpec `json:"aws,omitempty"`

	Docker *carenv1.DockerSpec `json:"docker,omitempty"`

	Nutanix *carenv1.NutanixSpec `json:"nutanix,omitempty"`

	carenv1.GenericClusterConfigSpec `json:",inline"`

	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`

	ControlPlane *ControlPlaneConfig `json:"controlPlane,omitempty"`
}

type ControlPlaneConfig struct {
	AWS *carenv1.AWSControlPlaneNodeSpec `json:"aws,omitempty"`

	Docker *carenv1.DockerNodeConfigSpec `json:"docker,omitempty"`

	Nutanix *carenv1.NutanixNodeConfigSpec `json:"nutanix,omitempty"`
}
