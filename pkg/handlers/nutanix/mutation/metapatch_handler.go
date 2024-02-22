// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	genericmutation "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPD clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := append(
		[]mutation.MetaMutator{},
		genericmutation.MetaMutators(mgr)...,
	)

	return mutation.NewMetaGeneratePatchesHandler(
		"nutanixClusterConfigPatch",
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPD workers.
func MetaWorkerPatchHandler() handlers.Named {
	patchHandlers := []mutation.MetaMutator{}

	return mutation.NewMetaGeneratePatchesHandler(
		"nutanixWorkerConfigPatch",
		patchHandlers...,
	)
}
