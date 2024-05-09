// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mutation

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers/mutation"
)

// MetaPatchHandler returns a meta patch handler for mutating generic Kubernetes clusters.
func MetaPatchHandler(mgr manager.Manager) handlers.Named {
	return mutation.NewMetaGeneratePatchesHandler(
		"genericClusterConfigPatch",
		mgr.GetClient(),
		MetaMutators(mgr)...,
	)
}
