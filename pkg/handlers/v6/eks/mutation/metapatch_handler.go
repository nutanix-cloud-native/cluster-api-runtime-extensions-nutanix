// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/ami"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/iaminstanceprofile"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/identityref"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/instancetype"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/network"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/placementgroup"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/placementgroupnfd"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/region"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/securitygroups"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/tags"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/eks/mutation/volumes"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPA clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	//nolint:prealloc // Only set up once on startup, prealloc is unnecessary.
	patchHandlers := []mutation.MetaMutator{
		region.NewPatch(),
		network.NewPatch(),
		identityref.NewPatch(),
		tags.NewClusterPatch(),
	}
	patchHandlers = append(patchHandlers, metaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"eksClusterv6configpatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	//nolint:prealloc // Only set up once on startup, prealloc is unnecessary.
	patchHandlers := []mutation.MetaMutator{
		iaminstanceprofile.NewWorkerPatch(),
		instancetype.NewWorkerPatch(),
		ami.NewWorkerPatch(),
		securitygroups.NewWorkerPatch(),
		volumes.NewWorkerPatch(),
		placementgroup.NewWorkerPatch(),
		placementgroupnfd.NewWorkerPatch(),
		tags.NewWorkerPatch(),
	}
	patchHandlers = append(patchHandlers, workerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"eksWorkerv6configpatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
