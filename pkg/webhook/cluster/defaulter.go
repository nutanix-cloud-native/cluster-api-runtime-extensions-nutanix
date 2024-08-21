// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewDefaulter(client ctrlclient.Client, decoder *admission.Decoder) admission.Handler {
	return admission.MultiMutatingHandler(
		NewClusterUUIDLabeler(client, decoder).Defaulter(),
	)
}
