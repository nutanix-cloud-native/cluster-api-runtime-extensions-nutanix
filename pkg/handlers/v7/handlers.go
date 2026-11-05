// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v7

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	awsmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/aws/mutation"
	dockermutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/docker/mutation"
	eksmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/eks/mutation"
	genericmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/generic/mutation"
	nutanixmutation "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7/nutanix/mutation"
)

// AllHandlers returns the v7 topology mutation handlers.
func AllHandlers(mgr manager.Manager) []handlers.Named {
	return []handlers.Named{
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
