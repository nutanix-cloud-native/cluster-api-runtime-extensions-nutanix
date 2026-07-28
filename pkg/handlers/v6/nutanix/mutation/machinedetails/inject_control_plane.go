// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

func NewControlPlanePatch() *nutanixMachineDetailsPatchHandler {
	return newNutanixMachineDetailsPatchHandler(
		v1alpha1.ClusterConfigVariableName,
		[]string{
			v1alpha1.ControlPlaneConfigVariableName,
			v1alpha1.NutanixVariableName,
			VariableName,
		},
		selectors.InfrastructureControlPlaneMachines(
			"v1beta1",
			"NutanixMachineTemplate",
		),
	)
}
