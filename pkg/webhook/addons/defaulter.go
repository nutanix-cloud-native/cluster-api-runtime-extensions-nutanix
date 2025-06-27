// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/webhook/addons/registry"
)

func NewDefaulter(client ctrlclient.Client, decoder admission.Decoder) admission.Handler {
	return admission.MultiMutatingHandler(
		allHandlers(client, decoder)...,
	)
}

// allHandlers returns a list of all defaulter handlers that should be registered,
// including any feature-gated handlers.
func allHandlers(
	client ctrlclient.Client, decoder admission.Decoder,
) []admission.Handler {
	var handlers []admission.Handler
	if feature.Gates.Enabled(feature.AutoEnableWorkloadClusterRegistry) {
		handlers = append(handlers, registry.NewWorkloadClusterAutoEnabler(client, decoder).Defaulter())
	}
	return handlers
}
