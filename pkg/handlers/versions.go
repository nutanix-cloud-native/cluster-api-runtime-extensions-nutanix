// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/handlers"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v6"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/v7"
)

// AllHandlers returns the variable discovery and topology mutation handlers of every served version.
func AllHandlers(mgr manager.Manager) []handlers.Named {
	return slices.Concat(
		v6.AllHandlers(mgr),
		v7.AllHandlers(mgr),
		// new-version:entries
	)
}
