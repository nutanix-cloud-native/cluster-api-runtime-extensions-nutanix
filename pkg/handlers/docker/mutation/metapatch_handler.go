// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"os"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/mutation/customimage"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation"
)

// CARENSkipControlPlanePatchesEnv is the environment variable to disable control plane
// patches for the Docker cluster (e.g. to test if "KubeadmConfig is not up-to-date"
// is caused by CAREN). Set to "true" or "1" to skip. Used for e2e debugging only.
const CARENSkipControlPlanePatchesEnv = "CAREN_SKIP_CONTROL_PLANE_PATCHES"

func skipControlPlanePatches() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(CARENSkipControlPlanePatchesEnv)))
	return v == "true" || v == "1"
}

// MetaPatchHandler returns a meta patch handler for mutating CAPD clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		customimage.NewControlPlanePatch(),
	}
	patchHandlers = append(patchHandlers, genericmutation.MetaMutators(mgr)...)
	if !skipControlPlanePatches() {
		patchHandlers = append(patchHandlers, genericmutation.ControlPlaneMetaMutators()...)
	}

	return mutation.NewMetaGeneratePatchesHandler(
		"dockerClusterv5configpatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPD workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		customimage.NewWorkerPatch(),
	}
	patchHandlers = append(patchHandlers, genericmutation.WorkerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"dockerWorkerv5configpatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
