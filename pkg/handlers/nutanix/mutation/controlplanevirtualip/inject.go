// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/controlplanevirtualip"
)

func NewPatch(cl client.Client) *controlplanevirtualip.ControlPlaneVirtualIP {
	return controlplanevirtualip.NewControlPlaneVirtualIP(
		cl,
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.NutanixVariableName,
		controlplanevirtualip.VariableName,
	)
}
