// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	nutanixmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation"
	nutanixworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	nutanixmutationvprev "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v4/nutanix/mutation"
)

type Handlers struct{}

func New(
	_ *options.GlobalOptions,
) *Handlers {
	return &Handlers{}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		nutanixclusterconfig.NewVariable(),
		nutanixworkerconfig.NewVariable(),
		nutanixmutation.MetaPatchHandler(mgr),
		nutanixmutationvprev.MetaPatchHandler(mgr),
		nutanixmutation.MetaWorkerPatchHandler(mgr),
		nutanixmutationvprev.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(_ *pflag.FlagSet) {}
