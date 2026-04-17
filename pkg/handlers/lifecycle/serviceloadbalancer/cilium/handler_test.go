// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package cilium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer"
)

func TestNewReturnsProviderImpl(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().Build()
	var p serviceloadbalancer.ServiceLoadBalancerProvider = New(c, &Config{}, nil)
	assert.NotNil(t, p)
}
