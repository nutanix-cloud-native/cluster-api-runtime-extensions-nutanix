// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/controlplanevirtualip"
)

func NewPatch() *controlplanevirtualip.ControlPlaneVirtualIP {
	return controlplanevirtualip.NewControlPlaneVirtualIP(
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.NutanixVariableName,
		controlplanevirtualip.VariableName,
	)
}
