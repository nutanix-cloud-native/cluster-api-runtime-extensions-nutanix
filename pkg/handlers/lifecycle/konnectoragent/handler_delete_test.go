// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	testDeleteClusterName      = "test-cluster"
	testDeleteClusterNamespace = "default"
	testDeleteClusterUUID      = "cluster-uuid-1"
)

func deleteTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(clusterv1beta2.AddToScheme(scheme))
	utilruntime.Must(caaphv1.AddToScheme(scheme))
	return scheme
}

func registeredInPC(
	context.Context,
	ctrlclient.Client,
	*clusterv1beta2.Cluster,
	logr.Logger,
) (bool, error) {
	return true, nil
}

func testKonnectorCluster(t *testing.T, extra ...func(*clusterv1beta2.Cluster)) *clusterv1beta2.Cluster {
	t.Helper()

	cluster := &clusterv1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testDeleteClusterName,
			Namespace: testDeleteClusterNamespace,
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: testDeleteClusterUUID,
			},
		},
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				ClassRef: clusterv1beta2.ClusterClassRef{Name: "dummy-class"},
				Variables: []clusterv1beta2.ClusterVariable{{
					Name: v1alpha1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{Raw: []byte(
						`{"addons":{"konnectorAgent":{"strategy":"HelmAddon"}}}`,
					)},
				}},
			},
		},
		Status: clusterv1beta2.ClusterStatus{
			Conditions: []metav1.Condition{{
				Type:   clusterv1beta2.ClusterControlPlaneInitializedCondition,
				Status: metav1.ConditionTrue,
			}},
			Phase: string(clusterv1beta2.ClusterPhaseProvisioned),
		},
	}
	for _, opt := range extra {
		opt(cluster)
	}
	return cluster
}

func testHelmChartProxy(t *testing.T, deletionAge time.Duration) *caaphv1.HelmChartProxy {
	t.Helper()

	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       defaultHelmReleaseName + "-" + testDeleteClusterUUID,
			Namespace:  testDeleteClusterNamespace,
			Finalizers: []string{"test.finalizer"},
		},
	}
	if deletionAge > 0 {
		deletedAt := metav1.NewTime(time.Now().Add(-deletionAge))
		hcp.DeletionTimestamp = &deletedAt
	}
	return hcp
}

func callBeforeClusterDelete(
	t *testing.T,
	cluster *clusterv1beta2.Cluster,
	objects []ctrlclient.Object,
) *runtimehooksv1.BeforeClusterDeleteResponse {
	t.Helper()

	scheme := deleteTestScheme(t)
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()

	handler := &DefaultKonnectorAgent{
		client:                     fakeClient,
		config:                     NewConfig(&options.GlobalOptions{}),
		helmChartInfoGetter:        &config.HelmChartGetter{},
		variableName:               v1alpha1.ClusterConfigVariableName,
		variablePath:               []string{"addons", v1alpha1.KonnectorAgentVariableName},
		clusterRegistrationChecker: registeredInPC,
	}

	v1beta1Cluster := &clusterv1beta1.Cluster{}
	require.NoError(t, v1beta1Cluster.ConvertFrom(cluster.DeepCopy()))

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(
		context.Background(),
		&runtimehooksv1.BeforeClusterDeleteRequest{Cluster: *v1beta1Cluster},
		resp,
	)
	return resp
}

func TestBeforeClusterDelete_SkipsWhenAddonNotSpecified(t *testing.T) {
	t.Parallel()

	cluster := testKonnectorCluster(t, func(c *clusterv1beta2.Cluster) {
		c.Spec.Topology.Variables = []clusterv1beta2.ClusterVariable{{
			Name:  v1alpha1.ClusterConfigVariableName,
			Value: apiextensionsv1.JSON{Raw: []byte(`{"addons":{}}`)},
		}}
	})

	resp := callBeforeClusterDelete(t, cluster, []ctrlclient.Object{cluster})

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
}

func TestBeforeClusterDelete_CleanupCompleted(t *testing.T) {
	t.Parallel()

	cluster := testKonnectorCluster(t)
	resp := callBeforeClusterDelete(t, cluster, []ctrlclient.Object{cluster})

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
}

func TestBeforeClusterDelete_CleanupInProgressRetriesWithSuccess(t *testing.T) {
	t.Parallel()

	cluster := testKonnectorCluster(t)
	hcp := testHelmChartProxy(t, time.Minute)
	resp := callBeforeClusterDelete(t, cluster, []ctrlclient.Object{cluster, hcp})

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, beforeClusterDeleteRetryAfterSeconds, resp.RetryAfterSeconds)
	assert.Contains(t, resp.Message, "cleanup in progress")
}

func TestBeforeClusterDelete_CleanupTimedOutAllowsDeletion(t *testing.T) {
	t.Parallel()

	cluster := testKonnectorCluster(t)
	hcp := testHelmChartProxy(t, helmUninstallTimeout+time.Minute)
	resp := callBeforeClusterDelete(t, cluster, []ctrlclient.Object{cluster, hcp})

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
	assert.Contains(t, resp.Message, "timed out")
}

func TestBeforeClusterDelete_CleanupNotStartedRetriesWithSuccess(t *testing.T) {
	t.Parallel()

	cluster := testKonnectorCluster(t)
	hcp := testHelmChartProxy(t, 0)
	resp := callBeforeClusterDelete(t, cluster, []ctrlclient.Object{cluster, hcp})

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, beforeClusterDeleteRetryAfterSeconds, resp.RetryAfterSeconds)
	assert.Contains(t, resp.Message, "cleanup initiated")
}
