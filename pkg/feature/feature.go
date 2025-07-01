// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package feature

import (
	"k8s.io/component-base/featuregate"
)

const (
	AutoEnableWorkloadClusterRegistry  featuregate.Feature = "AutoEnableWorkloadClusterRegistry"
	SynchronizeWorkloadClusterRegistry featuregate.Feature = "SynchronizeWorkloadClusterRegistry"
)

// defaultFeatureGates returns all known feature gates.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout the codebase.
func defaultFeatureGates() map[featuregate.Feature]featuregate.FeatureSpec {
	return map[featuregate.Feature]featuregate.FeatureSpec{
		AutoEnableWorkloadClusterRegistry:  {Default: false, PreRelease: featuregate.Alpha},
		SynchronizeWorkloadClusterRegistry: {Default: false, PreRelease: featuregate.Alpha},
	}
}
