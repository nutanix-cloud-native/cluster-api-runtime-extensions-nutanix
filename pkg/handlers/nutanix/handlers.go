// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	deleteinv0280nutanixmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/deleteinv0280/nutanix/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/controlplanevirtualip"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	nutanixmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation"
	nutanixworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type Handlers struct {
	// kubeVIPConfig holds the configuration for the kube-vip control-plane virtual IP.
	controlPlaneVirtualIPConfig *controlplanevirtualip.Config
}

func New(
	globalOptions *options.GlobalOptions,
) *Handlers {
	return &Handlers{
		controlPlaneVirtualIPConfig: &controlplanevirtualip.Config{GlobalOptions: globalOptions},
	}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		nutanixclusterconfig.NewVariable(),
		nutanixworkerconfig.NewVariable(),
		nutanixmutation.MetaPatchHandler(mgr, h.controlPlaneVirtualIPConfig),
		deleteinv0280nutanixmutation.MetaPatchHandler(mgr, h.controlPlaneVirtualIPConfig),
		nutanixmutation.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.controlPlaneVirtualIPConfig.AddFlags("nutanix", flagSet)
}
