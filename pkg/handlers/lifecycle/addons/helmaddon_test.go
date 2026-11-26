// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/server"
)

func TestDefaultWaiterTimeoutFitsWithinLifecycleHookTimeout(t *testing.T) {
	t.Parallel()

	hookTimeout := time.Duration(server.LifecycleHookTimeoutSeconds) * time.Second
	assert.Less(
		t,
		defaultWaitTimeout,
		hookTimeout,
		"defaultWaitTimeout must finish before Cluster API's HTTP hook timeout cancels the request",
	)
	assert.Equal(t, 2*time.Second, defaultWaitInterval)
	assert.Equal(t, 20*time.Second, defaultWaitTimeout)
}

func TestWaitToBeReadyReturnsStructuredErrorOnContextTimeout(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, caaphv1.AddToScheme(scheme))

	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cilium-uuid-1",
			Namespace:  "default",
			Generation: 1,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(hcp).Build()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := waitToBeReady(ctx, cl, hcp)
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to wait for addon default/cilium-uuid-1 to deploy")
}
