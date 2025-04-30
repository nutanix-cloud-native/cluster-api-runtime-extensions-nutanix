// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package controlplanevirtualip

import (
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/controlplanevirtualip"
)

func NewPatch(
	cl ctrlclient.Client,
	cfg *controlplanevirtualip.Config,
) *controlplanevirtualip.ControlPlaneVirtualIP {
	return controlplanevirtualip.NewControlPlaneVirtualIP(
		cl,
		cfg,
		v1alpha1.ClusterConfigVariableName,
		v1alpha1.VsphereVariableName,
		controlplanevirtualip.VariableName,
	)
}
