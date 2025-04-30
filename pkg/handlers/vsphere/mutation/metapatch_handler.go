// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/controlplanevirtualip"
	vspherecontrolplanevirtualip "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/vsphere/mutation/controlplanevirtualip"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPX clusters.
func MetaPatchHandler(mgr manager.Manager, cfg *controlplanevirtualip.Config) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		vspherecontrolplanevirtualip.NewPatch(mgr.GetClient(), cfg),
	}
	patchHandlers = append(patchHandlers, genericmutation.MetaMutators(mgr)...)
	patchHandlers = append(patchHandlers, genericmutation.ControlPlaneMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"vsphereClusterV2ConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{}
	patchHandlers = append(patchHandlers, genericmutation.WorkerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"vsphereWorkerConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
