// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package features

import "k8s.io/component-base/featuregate"

// DefaultFeatureGates returns all known feature gates.
// To add a new feature, define a key for it above and add it here. The features will be
// available throughout the codebase.
func DefaultFeatureGates() map[featuregate.Feature]featuregate.FeatureSpec {
	return map[featuregate.Feature]featuregate.FeatureSpec{}
}
