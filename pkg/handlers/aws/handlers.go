// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"github.com/spf13/pflag"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/clusterconfig"
	awsmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/mutation"
	awsworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/aws/workerconfig"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
	v2awsmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v2/aws/mutation"
)

type Handlers struct{}

func New(
	_ *options.GlobalOptions,
) *Handlers {
	return &Handlers{}
}

func (h *Handlers) AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		awsclusterconfig.NewVariable(),
		awsworkerconfig.NewVariable(),
		awsmutation.MetaPatchHandler(mgr),
		v2awsmutation.MetaPatchHandler(mgr),
		awsmutation.MetaWorkerPatchHandler(mgr),
		v2awsmutation.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(_ *pflag.FlagSet) {}
