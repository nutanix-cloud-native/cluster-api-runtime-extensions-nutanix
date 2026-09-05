// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	capiutils "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/utils"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const testClusterUUID = "test-cluster-uuid"

func stubClusterRegisteredInPC(t *testing.T) {
	t.Helper()

	orig := lookupClusterRegistration
	lookupClusterRegistration = func(
		context.Context,
		ctrlclient.Client,
		*clusterv1beta2.Cluster,
		logr.Logger,
	) (bool, error) {
		return true, nil
	}
	t.Cleanup(func() {
		lookupClusterRegistration = orig
	})
}

func testClusterForDelete(t *testing.T) *clusterv1beta2.Cluster {
	t.Helper()

	return &clusterv1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: testClusterUUID,
			},
		},
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				ClassRef: clusterv1beta2.ClusterClassRef{Name: "dummy-class"},
				Variables: []clusterv1beta2.ClusterVariable{{
					Name: v1alpha1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{Raw: []byte(`{
						"nutanix": {
							"prismCentralEndpoint": {
								"url": "https://prism-central.example.com:9440",
								"insecure": true
							}
						},
						"addons": {
							"konnectorAgent": {
								"credentials": { "secretRef": {"name":"test-secret"} }
							}
						}
					}`)},
				}},
			},
		},
		Status: clusterv1beta2.ClusterStatus{
			Phase: string(clusterv1beta2.ClusterPhaseProvisioned),
			Conditions: []metav1.Condition{{
				Type:   clusterv1beta2.ClusterControlPlaneInitializedCondition,
				Status: metav1.ConditionTrue,
			}},
		},
	}
}

func stuckHelmChartProxy(t *testing.T) *caaphv1.HelmChartProxy {
	t.Helper()

	deletionTS := metav1.NewTime(time.Now().Add(-helmUninstallTimeout - time.Second))
	return &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:              fmt.Sprintf("%s-%s", defaultHelmReleaseName, testClusterUUID),
			Namespace:         "default",
			Finalizers:        []string{caaphv1.HelmChartProxyFinalizer},
			DeletionTimestamp: &deletionTS,
		},
		Spec: caaphv1.HelmChartProxySpec{
			ChartName:       defaultHelmReleaseName,
			RepoURL:         "https://example.invalid",
			ClusterSelector: metav1.LabelSelector{MatchLabels: map[string]string{"cluster.x-k8s.io/cluster-name": "test"}},
		},
	}
}

func TestCheckCleanupStatus_TimedOut(t *testing.T) {
	cluster := testClusterForDelete(t)
	hcp := stuckHelmChartProxy(t)
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(cluster, hcp).Build()
	handler := &DefaultKonnectorAgent{client: client}

	status, msg, err := handler.checkCleanupStatus(context.Background(), cluster, logr.Discard())
	require.NoError(t, err)
	assert.Equal(t, cleanupStatusTimedOut, status)
	assert.Contains(t, msg, "stuck finalizers")
}

func TestCheckCleanupStatus_InProgress(t *testing.T) {
	cluster := testClusterForDelete(t)
	hcp := stuckHelmChartProxy(t)
	recent := metav1.NewTime(time.Now().Add(-time.Second))
	hcp.DeletionTimestamp = &recent
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(cluster, hcp).Build()
	handler := &DefaultKonnectorAgent{client: client}

	status, _, err := handler.checkCleanupStatus(context.Background(), cluster, logr.Discard())
	require.NoError(t, err)
	assert.Equal(t, cleanupStatusInProgress, status)
}

func TestRemoveStuckHelmChartProxyFinalizers(t *testing.T) {
	cluster := testClusterForDelete(t)
	hcp := stuckHelmChartProxy(t)
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(cluster, hcp).Build()
	handler := &DefaultKonnectorAgent{client: client}

	require.NoError(t, handler.removeStuckHelmChartProxyFinalizers(
		context.Background(),
		cluster,
		logr.Discard(),
	))

	got := &caaphv1.HelmChartProxy{}
	err := client.Get(context.Background(), ctrlclient.ObjectKeyFromObject(hcp), got)
	if apierrors.IsNotFound(err) {
		return
	}
	require.NoError(t, err)
	assert.Empty(t, got.Finalizers)
}

func TestBeforeClusterDelete_TimedOutHelmUninstallAllowsDeletion(t *testing.T) {
	stubClusterRegisteredInPC(t)

	cluster := testClusterForDelete(t)
	hcp := stuckHelmChartProxy(t)
	client := fake.NewClientBuilder().WithScheme(testScheme).WithObjects(cluster, hcp).Build()
	handler := &DefaultKonnectorAgent{
		client:              client,
		config:              NewConfig(&options.GlobalOptions{}),
		helmChartInfoGetter: &config.HelmChartGetter{},
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.KonnectorAgentVariableName},
	}

	v1b1, err := capiutils.ConvertV1Beta2ClusterToV1Beta1(cluster)
	require.NoError(t, err)

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), &runtimehooksv1.BeforeClusterDeleteRequest{
		Cluster: clusterv1beta1.Cluster{
			ObjectMeta: v1b1.ObjectMeta,
			Spec:       v1b1.Spec,
		},
	}, resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Contains(t, resp.Message, "timed out")
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)

	got := &caaphv1.HelmChartProxy{}
	err = client.Get(context.Background(), ctrlclient.ObjectKeyFromObject(hcp), got)
	if apierrors.IsNotFound(err) {
		return
	}
	require.NoError(t, err)
	assert.Empty(t, got.Finalizers)
}
