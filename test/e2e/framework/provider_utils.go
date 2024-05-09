//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

// The functions in this file are used to get the provider names from the e2e config
// based on the provider type. They are adaptations of the functions that already exist
// on the E2EConfig type for other provider types, but unfortunately the existing functions
// do not include core, bootstrap, and control plane providers.

func CoreProvider(cfg *clusterctl.E2EConfig) string {
	return getProviders(cfg, clusterctlv1.CoreProviderType)[0]
}

func BootstrapProviders(cfg *clusterctl.E2EConfig) []string {
	return getProviders(cfg, clusterctlv1.BootstrapProviderType)
}

func ControlPlaneProviders(cfg *clusterctl.E2EConfig) []string {
	return getProviders(cfg, clusterctlv1.ControlPlaneProviderType)
}

func getProviders(cfg *clusterctl.E2EConfig, t clusterctlv1.ProviderType) []string {
	providers := []string{}
	for _, provider := range cfg.Providers {
		if provider.Type == string(t) {
			providers = append(providers, provider.Name)
		}
	}
	return providers
}
