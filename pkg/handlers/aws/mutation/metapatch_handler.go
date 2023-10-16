// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers"
	"github.com/d2iq-labs/capi-runtime-extensions/common/pkg/capi/clustertopology/handlers/mutation"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/ami"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/cni/calico"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/iaminstanceprofile"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/instancetype"
	"github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/aws/mutation/region"
	genericmutation "github.com/d2iq-labs/capi-runtime-extensions/pkg/handlers/generic/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating CAPA clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	patchHandlers := append(
		[]mutation.MetaMutator{
			region.NewPatch(),
			calico.NewPatch(),
			iaminstanceprofile.NewControlPlanePatch(),
			instancetype.NewControlPlanePatch(),
			ami.NewControlPlanePatch(),
		},
		genericmutation.MetaMutators(mgr)...,
	)

	return mutation.NewMetaGeneratePatchesHandler(
		"awsClusterConfigPatch",
		patchHandlers...,
	)
}

// MetaWorkerPatchHandler returns a meta patch handler for mutating CAPA workers.
func MetaWorkerPatchHandler() handlers.Named {
	patchHandlers := []mutation.MetaMutator{
		iaminstanceprofile.NewWorkerPatch(),
		instancetype.NewWorkerPatch(),
		ami.NewWorkerPatch(),
	}

	return mutation.NewMetaGeneratePatchesHandler(
		"awsWorkerConfigPatch",
		patchHandlers...,
	)
}
