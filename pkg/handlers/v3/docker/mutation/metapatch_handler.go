// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/mutation/customimage"
	genericmutationvprev "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v3/generic/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPD clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		customimage.NewControlPlanePatch(),
	}
	patchHandlers = append(patchHandlers, genericmutationvprev.MetaMutators(mgr)...)
	patchHandlers = append(patchHandlers, genericmutationvprev.ControlPlaneMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"dockerClusterV3ConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPD workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		customimage.NewWorkerPatch(),
	}
	patchHandlers = append(patchHandlers, genericmutationvprev.WorkerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"dockerWorkerV3ConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
