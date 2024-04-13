// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/mutation/customimage"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/mutation/dockerapiservercertsans"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPD clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := append(
		[]mutation.MetaMutator{
			customimage.NewControlPlanePatch(),
			dockerapiservercertsans.NewPatch(),
		},
		genericmutation.MetaMutators(mgr)...,
	)

	return mutation.NewMetaGeneratePatchesHandler(
		"dockerClusterConfigPatch",
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPD workers.
func MetaWorkerPatchHandler() handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		customimage.NewWorkerPatch(),
	}

	return mutation.NewMetaGeneratePatchesHandler(
		"dockerWorkerConfigPatch",
		patchHandlers...,
	)
}
