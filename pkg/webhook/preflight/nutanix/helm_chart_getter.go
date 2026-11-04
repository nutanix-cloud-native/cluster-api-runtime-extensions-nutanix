// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix

import (
	"sync"

	lifecycleconfig "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
)

var (
	helmChartGetterMu sync.RWMutex
	helmChartGetter   *lifecycleconfig.HelmChartGetter
)

// InitHelmChartGetter wires the same Helm chart ConfigMap reader used by lifecycle handlers.
// Must be called from main after flags are parsed and before the webhook server starts.
func InitHelmChartGetter(getter *lifecycleconfig.HelmChartGetter) {
	helmChartGetterMu.Lock()
	defer helmChartGetterMu.Unlock()
	helmChartGetter = getter
}

// GetHelmChartGetter returns the Helm chart ConfigMap reader wired from main (may be nil before InitHelmChartGetter).
func GetHelmChartGetter() *lifecycleconfig.HelmChartGetter {
	helmChartGetterMu.RLock()
	defer helmChartGetterMu.RUnlock()
	return helmChartGetter
}
