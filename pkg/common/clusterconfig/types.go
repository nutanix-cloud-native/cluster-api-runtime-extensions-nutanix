// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package clusterconfig

import carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"

type ClusterConfig struct {
	AWS *carenv1.AWSSpec `json:"aws,omitempty"`

	Docker *carenv1.AWSSpec `json:"doker,omitempty"`

	Nutanix *carenv1.NutanixSpec `json:"nutanix,omitempty"`

	carenv1.GenericClusterConfigSpec `json:",inline"`
}
