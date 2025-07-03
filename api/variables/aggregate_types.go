// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package variables

import carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"

// The types here are to be used internally to simplify the handling of cluster configurations for different
// providers. It is not meant to be used as a CRD.
// By including all the possible configurations for all the providers, we can easily switch between providers in code
// without type assertions/switches and avoids passing around `interface{}` or `any` types.
// Every provider-specific cluster config variable will successfully unmarshal to this type and so it is safe to use
// this internally when a handler provides functionality for multiple providers but exhibits different behaviour per
// provider.

type ClusterConfigSpec struct {
	AWS *carenv1.AWSSpec `json:"aws,omitempty"`

	Docker *carenv1.DockerSpec `json:"docker,omitempty"`

	Nutanix *carenv1.NutanixSpec `json:"nutanix,omitempty"`

	carenv1.GenericClusterConfigSpec `json:",inline"`

	Addons *Addons `json:"addons,omitempty"`

	ControlPlane *ControlPlaneSpec `json:"controlPlane,omitempty"`

	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

type ControlPlaneSpec struct {
	AWS *carenv1.AWSControlPlaneNodeSpec `json:"aws,omitempty"`

	Docker *carenv1.DockerNodeSpec `json:"docker,omitempty"`

	Nutanix *carenv1.NutanixControlPlaneNodeSpec `json:"nutanix,omitempty"`

	carenv1.GenericControlPlaneSpec `json:",inline"`

	carenv1.GenericNodeSpec `json:",inline"`
}

type WorkerNodeConfigSpec struct {
	AWS *carenv1.AWSWorkerNodeSpec `json:"aws,omitempty"`

	Docker *carenv1.DockerNodeSpec `json:"docker,omitempty"`

	Nutanix *carenv1.NutanixWorkerNodeSpec `json:"nutanix,omitempty"`

	carenv1.GenericNodeSpec `json:",inline"`
}

type Addons struct {
	carenv1.GenericAddons `json:",inline"`

	CSI *CSI `json:"csi,omitempty"`

	COSI *COSI `json:"cosi,omitempty"`
}

type CSI struct {
	carenv1.GenericCSI `json:",inline"`

	Providers map[string]carenv1.CSIProvider `json:"providers"`
}

type COSI struct {
	carenv1.GenericCOSI `json:",inline"`
}
