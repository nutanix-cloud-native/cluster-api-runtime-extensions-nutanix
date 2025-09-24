// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/ami"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/iaminstanceprofile"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/identityref"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/instancetype"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/network"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/placementgroup"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/region"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/securitygroups"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/eks/mutation/volumes"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPA clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		region.NewPatch(),
		network.NewPatch(),
		identityref.NewPatch(),
	}
	patchHandlers = append(patchHandlers, metaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"eksClusterV4ConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		iaminstanceprofile.NewWorkerPatch(),
		instancetype.NewWorkerPatch(),
		ami.NewWorkerPatch(),
		securitygroups.NewWorkerPatch(),
		volumes.NewWorkerPatch(),
		placementgroup.NewWorkerPatch(),
	}
	patchHandlers = append(patchHandlers, workerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"eksWorkerV4ConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
