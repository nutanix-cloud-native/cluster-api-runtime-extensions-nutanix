// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/common/controlplaneendpoint/virtualip"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/clusterconfig"
	nutanixmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/mutation"
	nutanixworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/nutanix/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

type Handlers struct {
	// kubeVIPConfig holds the configuration for the kube-vip control-plane virtual IP.
	kubeVIPConfig *virtualip.KubeVIPFromConfigMapConfig
}

func New(
	globalOptions *options.GlobalOptions,
) *Handlers {
	return &Handlers{
		kubeVIPConfig: &virtualip.KubeVIPFromConfigMapConfig{GlobalOptions: globalOptions},
	}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	virtualIPProvider := virtualip.NewKubeVIPFromConfigMapProvider(mgr.GetClient(), h.kubeVIPConfig)
	return []handlers.Named{
		nutanixclusterconfig.NewVariable(),
		nutanixworkerconfig.NewVariable(),
		nutanixmutation.MetaPatchHandler(mgr, virtualIPProvider),
		nutanixmutation.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(flagSet *pflag.FlagSet) {
	h.kubeVIPConfig.AddFlags("nutanix", flagSet)
}
