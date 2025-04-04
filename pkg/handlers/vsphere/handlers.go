// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/controlplanevirtualip"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	vsphereclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/vsphere/clusterconfig"

	vspheremutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/vsphere/mutation"
	vsphereworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/vsphere/workerconfig"
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
		vsphereclusterconfig.NewVariable(),
		vsphereworkerconfig.NewVariable(),
		vspheremutation.MetaPatchHandler(mgr, h.controlPlaneVirtualIPConfig),
		vspheremutation.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.controlPlaneVirtualIPConfig.AddFlags("vsphere", flagSet)
}
