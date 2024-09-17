// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/controlplanevirtualip"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/controlplaneendpoint"
	nutanixcontrolplanevirtualip "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/controlplanevirtualip"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/machinedetails"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/prismcentralendpoint"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPX clusters.
func MetaPatchHandler(mgr manager.Manager, cfg *controlplanevirtualip.Config) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		controlplaneendpoint.NewPatch(),
		nutanixcontrolplanevirtualip.NewPatch(mgr.GetClient(), cfg),
		prismcentralendpoint.NewPatch(),
		machinedetails.NewControlPlanePatch(),
	}
	patchHandlers = append(patchHandlers, genericmutation.MetaMutators(mgr)...)
	patchHandlers = append(patchHandlers, genericmutation.ControlPlaneMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"nutanixClusterConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		machinedetails.NewWorkerPatch(),
	}
	patchHandlers = append(patchHandlers, genericmutation.WorkerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"nutanixWorkerConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
