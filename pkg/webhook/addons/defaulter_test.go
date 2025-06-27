// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"testing"

	"github.com/stretchr/testify/assert"
	featuregatetesting "k8s.io/component-base/featuregate/testing"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/feature"
)

// Test_allHandlers is a crude test to ensure handlers are only registered when the feature gate is enabled.
func Test_allHandlers(t *testing.T) {
	handlers := allHandlers(nil, nil)
	assert.Empty(t, handlers)

	// Enable the feature gates and test again.
	featuregatetesting.SetFeatureGateDuringTest(
		t,
		feature.Gates,
		feature.AutoEnableWorkloadClusterRegistry,
		true,
	)
	handlers = allHandlers(nil, nil)
	assert.Len(t, handlers, 1)
}
