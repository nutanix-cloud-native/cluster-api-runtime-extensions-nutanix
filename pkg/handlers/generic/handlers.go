// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package generic

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	genericclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/clusterconfig"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation"
	v2genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v2/generic/mutation"
)

type Handlers struct{}

func New() *Handlers {
	return &Handlers{}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		genericclusterconfig.NewVariable(),
		genericmutation.MetaPatchHandler(mgr),
		v2genericmutation.MetaPatchHandler(mgr),
		genericmutation.MetaWorkerPatchHandler(mgr),
		v2genericmutation.MetaWorkerPatchHandler(mgr),
	}
}
