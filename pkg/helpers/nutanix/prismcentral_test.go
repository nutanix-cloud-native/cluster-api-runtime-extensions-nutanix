// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package nutanix_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	nutanix "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/helpers/nutanix"
)

func TestPrismCentralVersionForCluster_DoesNotUsePreflightSkipSentinel(t *testing.T) {
	t.Parallel()

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				carenv1.PreflightChecksSkipAnnotationKey: carenv1.PreflightChecksSkipAllAnnotationValue,
			},
		},
	}

	version, err := nutanix.PrismCentralVersionForCluster(context.Background(), nil, cluster, nil)

	require.NoError(t, err)
	assert.Empty(t, version)
}
