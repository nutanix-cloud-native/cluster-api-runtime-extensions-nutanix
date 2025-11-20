// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/clusterconfig"
	dockermutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/mutation"
	dockerworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/docker/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	dockermutationvprev "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v4/docker/mutation"
)

type Handlers struct{}

func New(
	_ *options.GlobalOptions,
) *Handlers {
	return &Handlers{}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		dockerclusterconfig.NewVariable(),
		dockerworkerconfig.NewVariable(),
		dockermutation.MetaPatchHandler(mgr),
		dockermutationvprev.MetaPatchHandler(mgr),
		dockermutation.MetaWorkerPatchHandler(mgr),
		dockermutationvprev.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(_ *pflag.FlagSet) {}
