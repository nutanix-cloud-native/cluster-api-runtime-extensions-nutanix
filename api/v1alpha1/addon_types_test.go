// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import "testing"

func TestServiceLoadBalancerProviders(t *testing.T) {
	t.Parallel()

	providers := map[string]string{
		"MetalLB": ServiceLoadBalancerProviderMetalLB,
		"Cilium":  ServiceLoadBalancerProviderCilium,
	}

	seen := make(map[string]string, len(providers))
	for want, got := range providers {
		if got != want {
			t.Errorf("provider constant for %q = %q, want %q", want, got, want)
		}
		if existing, dup := seen[got]; dup {
			t.Errorf("provider value %q used by both %q and %q", got, existing, want)
		}
		seen[got] = want
	}
}
