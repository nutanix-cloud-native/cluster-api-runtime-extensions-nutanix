// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package feature

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

//nolint:gochecknoinits // Code is copied from upstream.
func init() {
	runtime.Must(MutableGates.Add(defaultFeatureGates()))
}

var (
	// MutableGates is a mutable version of DefaultFeatureGate.
	// Only top-level commands/options setup should make use of this.
	// Tests that need to modify feature gates for the duration of their test should use:
	//   featuregatetesting "k8s.io/component-base/featuregate/testing"
	//   featuregatetesting.SetFeatureGateDuringTest(
	//     t,
	//     features.Gates,
	//     features.<FeatureName>,
	//     <value>,
	//   )()
	MutableGates featuregate.MutableFeatureGate = featuregate.NewFeatureGate()

	// Gates is a shared global FeatureGate.
	Gates featuregate.FeatureGate = MutableGates
)
