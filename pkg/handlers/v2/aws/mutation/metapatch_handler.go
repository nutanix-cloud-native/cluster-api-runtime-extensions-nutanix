// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/ami"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/cni/calico"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/controlplaneloadbalancer"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/iaminstanceprofile"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/instancetype"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/network"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/region"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation/securitygroups"
	genericmutationv2 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v2/generic/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPA clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		calico.NewPatch(),
		region.NewPatch(),
		network.NewPatch(),
		controlplaneloadbalancer.NewPatch(),
		iaminstanceprofile.NewControlPlanePatch(),
		instancetype.NewControlPlanePatch(),
		ami.NewControlPlanePatch(),
		securitygroups.NewControlPlanePatch(),
	}
	patchHandlers = append(patchHandlers, genericmutationv2.MetaMutators(mgr)...)
	patchHandlers = append(patchHandlers, genericmutationv2.ControlPlaneMetaMutators()...)

	return mutation.NewMetaGeneratePatchesHandler(
		"awsClusterV2ConfigPatch",
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
	}
	patchHandlers = append(patchHandlers, genericmutationv2.WorkerMetaMutators()...)

	// The previous handler did not have "v2" in the name.
	return mutation.NewMetaGeneratePatchesHandler(
		"awsWorkerConfigPatch",
		mgr.GetClient(),
		patchHandlers...,
	)
}
