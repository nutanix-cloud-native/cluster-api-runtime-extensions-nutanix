// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v7

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	awsclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/aws/clusterconfig"
	awsmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/aws/mutation"
	awsworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/aws/workerconfig"
	dockerclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/docker/clusterconfig"
	dockermutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/docker/mutation"
	dockerworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/docker/workerconfig"
	eksclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/eks/clusterconfig"
	eksmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/eks/mutation"
	eksworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/eks/workerconfig"
	genericclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/generic/clusterconfig"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/generic/mutation"
	nutanixclusterconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/nutanix/clusterconfig"
	nutanixmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/nutanix/mutation"
	nutanixworkerconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/nutanix/workerconfig"
)

// AllHandlers returns the v7 variable discovery and topology mutation handlers.
func AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
		awsclusterconfig.NewVariable(),
		awsworkerconfig.NewVariable(),
		dockerclusterconfig.NewVariable(),
		dockerworkerconfig.NewVariable(),
		eksclusterconfig.NewVariable(),
		eksworkerconfig.NewVariable(),
		genericclusterconfig.NewVariable(),
		nutanixclusterconfig.NewVariable(),
		nutanixworkerconfig.NewVariable(),
		awsmutation.MetaPatchHandler(mgr),
		awsmutation.MetaWorkerPatchHandler(mgr),
		dockermutation.MetaPatchHandler(mgr),
		dockermutation.MetaWorkerPatchHandler(mgr),
		eksmutation.MetaPatchHandler(mgr),
		eksmutation.MetaWorkerPatchHandler(mgr),
		genericmutation.MetaPatchHandler(mgr),
		genericmutation.MetaWorkerPatchHandler(mgr),
		nutanixmutation.MetaPatchHandler(mgr),
		nutanixmutation.MetaWorkerPatchHandler(mgr),
	}
}
