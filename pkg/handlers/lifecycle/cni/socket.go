// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cni

import (
	"fmt"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

// ReadinessSocketPath returns the readiness socket path for the given CNI provider.
// The socket path is used by Multus to wait for the primary CNI to be ready.
// Returns an empty string and error for unsupported providers.
func ReadinessSocketPath(provider string) (string, error) {
	switch provider {
	case v1alpha1.CNIProviderCilium:
		return "/run/cilium/cilium.sock", nil
	case v1alpha1.CNIProviderCalico:
		return "/var/run/calico/cni-server.sock", nil
	default:
		return "", fmt.Errorf("could not determine CNI socket, unsupported provider: %s", provider)
	}
}
