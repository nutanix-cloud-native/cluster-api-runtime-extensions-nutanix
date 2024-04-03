// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	genericmutation "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/controlplaneendpoint"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/machinedetails"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation/prismcentralendpoint"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPX clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := append(
		[]mutation.MetaMutator{
			controlplaneendpoint.NewPatch(),
			prismcentralendpoint.NewPatch(),
			machinedetails.NewControlPlanePatch(),
		},
		genericmutation.MetaMutators(mgr)...,
	)

	return mutation.NewMetaGeneratePatchesHandler(
		"nutanixClusterConfigPatch",
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler() handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		machinedetails.NewWorkerPatch(),
	}

	return mutation.NewMetaGeneratePatchesHandler(
		"nutanixWorkerConfigPatch",
		patchHandlers...,
	)
}
