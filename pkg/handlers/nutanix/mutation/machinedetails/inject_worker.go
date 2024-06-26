// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package machinedetails

import (
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/patches/selectors"
)

func NewWorkerPatch() *nutanixMachineDetailsPatchHandler {
	return newNutanixMachineDetailsPatchHandler(
		v1alpha1.WorkerConfigVariableName,
		[]string{
			v1alpha1.NutanixVariableName,
			VariableName,
		},
		selectors.InfrastructureWorkerMachineTemplates(
			"v1beta1",
			"NutanixMachineTemplate",
		),
	)
}
