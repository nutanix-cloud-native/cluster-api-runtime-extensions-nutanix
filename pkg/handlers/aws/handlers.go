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
	deleteinv0280awsmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/deleteinv0280/aws/mutation"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
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
		deleteinv0280awsmutation.MetaPatchHandler(mgr),
		awsmutation.MetaWorkerPatchHandler(mgr),
	}
}

func (h *Handlers) AddFlags(_ *pflag.FlagSet) {}
