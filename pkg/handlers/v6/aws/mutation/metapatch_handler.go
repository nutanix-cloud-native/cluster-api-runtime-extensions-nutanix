// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/ami"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/cni/calico"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/controlplaneloadbalancer"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/iaminstanceprofile"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/identityref"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/instancetype"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/network"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/placementgroup"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/placementgroupnfd"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/region"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/securitygroups"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/tags"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/aws/mutation/volumes"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6/generic/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPA clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	//nolint:prealloc // Only set up once on startup, prealloc is unnecessary.
	patchHandlers := []mutation.MetaMutator{
		calico.NewPatch(),
		tags.NewClusterPatch(),
		region.NewPatch(),
		network.NewPatch(),
		controlplaneloadbalancer.NewPatch(),
		tags.NewControlPlanePatch(),
		identityref.NewPatch(),
		iaminstanceprofile.NewControlPlanePatch(),
		instancetype.NewControlPlanePatch(),
		ami.NewControlPlanePatch(),
		securitygroups.NewControlPlanePatch(),
		volumes.NewControlPlanePatch(),
		placementgroup.NewControlPlanePatch(),
		placementgroupnfd.NewControlPlanePatch(),
	}
	patchHandlers = append(patchHandlers, genericmutation.MetaMutators(mgr)...)
	patchHandlers = append(patchHandlers, genericmutation.ControlPlaneMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"awsClusterv6configpatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler(mgr manager.Manager) handlers.Named {
	//nolint:prealloc // Only set up once on startup, prealloc is unnecessary.
	patchHandlers := []mutation.MetaMutator{
		tags.NewWorkerPatch(),
		iaminstanceprofile.NewWorkerPatch(),
		instancetype.NewWorkerPatch(),
		ami.NewWorkerPatch(),
		securitygroups.NewWorkerPatch(),
		volumes.NewWorkerPatch(),
		placementgroup.NewWorkerPatch(),
		placementgroupnfd.NewWorkerPatch(),
	}
	patchHandlers = append(patchHandlers, genericmutation.WorkerMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"awsWorkerv6configpatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
