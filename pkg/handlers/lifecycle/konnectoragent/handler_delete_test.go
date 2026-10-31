// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	deleteTestClusterName = "nkp-manoj-75"
	deleteTestNamespace   = "kommander"
	deleteTestClusterUUID = "059f3c38-c6ef-4101-acfc-b8288d98c3b5"
)

func deleteTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(clusterv1beta2.AddToScheme(scheme))
	utilruntime.Must(caaphv1.AddToScheme(scheme))
	return scheme
}

func provisionedClusterWithKonnector() *clusterv1beta2.Cluster {
	return &clusterv1beta2.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deleteTestClusterName,
			Namespace: deleteTestNamespace,
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: deleteTestClusterUUID,
			},
		},
		Spec: clusterv1beta2.ClusterSpec{
			Topology: clusterv1beta2.Topology{
				ClassRef: clusterv1beta2.ClusterClassRef{Name: "nutanix-quick-start"},
				Variables: []clusterv1beta2.ClusterVariable{{
					Name: v1alpha1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{
						Raw: []byte(`{"addons":{"konnectorAgent":{}}}`),
					},
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

func konnectorHelmChartProxy(deleting bool) *caaphv1.HelmChartProxy {
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, deleteTestClusterUUID),
			Namespace: deleteTestNamespace,
		},
	}
	if deleting {
		now := metav1.NewTime(time.Now().Add(-time.Minute))
		hcp.DeletionTimestamp = &now
		hcp.Finalizers = []string{"test.konnector/cleanup"}
	}
	return hcp
}

func registrationConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultClusterRegistrationConfigMapName,
			Namespace: defaultHelmReleaseNamespace,
		},
	}
}

func newDeleteHandler(
	t *testing.T,
	mgmtObjects []ctrlclient.Object,
	getter func(context.Context, string, ctrlclient.Client, ctrlclient.ObjectKey) (ctrlclient.Client, error),
) *DefaultKonnectorAgent {
	t.Helper()

	mgmtClient := fake.NewClientBuilder().
		WithScheme(deleteTestScheme()).
		WithStatusSubresource(&clusterv1beta2.Cluster{}).
		WithObjects(mgmtObjects...).
		Build()

	return &DefaultKonnectorAgent{
		client:              mgmtClient,
		config:              NewConfig(&options.GlobalOptions{}),
		helmChartInfoGetter: &config.HelmChartGetter{},
		clusterClientGetter: getter,
		variableName:        v1alpha1.ClusterConfigVariableName,
		variablePath:        []string{"addons", v1alpha1.KonnectorAgentVariableName},
	}
}

func deleteRequest() *runtimehooksv1.BeforeClusterDeleteRequest {
	cluster := provisionedClusterWithKonnector()
	return &runtimehooksv1.BeforeClusterDeleteRequest{Cluster: *cluster}
}

func TestBeforeClusterDelete_RemoteClientFails(t *testing.T) {
	t.Parallel()

	getterCalls := 0
	handler := newDeleteHandler(t, []ctrlclient.Object{
		provisionedClusterWithKonnector(),
		konnectorHelmChartProxy(false),
	}, func(context.Context, string, ctrlclient.Client, ctrlclient.ObjectKey) (ctrlclient.Client, error) {
		getterCalls++
		return nil, fmt.Errorf("kubeconfig secret missing")
	})

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), deleteRequest(), resp)

	assert.Equal(t, 1, getterCalls)
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
	assertHelmChartProxyPresent(t, handler.client)
}

func TestBeforeClusterDelete_ConfigMapGetFails(t *testing.T) {
	t.Parallel()

	remoteClient := fake.NewClientBuilder().
		WithScheme(deleteTestScheme()).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				context.Context,
				ctrlclient.WithWatch,
				ctrlclient.ObjectKey,
				ctrlclient.Object,
				...ctrlclient.GetOption,
			) error {
				return fmt.Errorf("temporary api failure")
			},
		}).
		Build()

	handler := newDeleteHandler(t, []ctrlclient.Object{
		provisionedClusterWithKonnector(),
		konnectorHelmChartProxy(false),
	}, func(context.Context, string, ctrlclient.Client, ctrlclient.ObjectKey) (ctrlclient.Client, error) {
		return remoteClient, nil
	})

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), deleteRequest(), resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
	assert.Empty(t, resp.Message)
	assertHelmChartProxyPresent(t, handler.client)
}

func TestBeforeClusterDelete_ConfigMapVanishesMidCleanup(t *testing.T) {
	t.Parallel()

	t.Run("registered cluster keeps waiting for helm cleanup", func(t *testing.T) {
		t.Parallel()

		remoteClient := fake.NewClientBuilder().
			WithScheme(deleteTestScheme()).
			WithObjects(registrationConfigMap()).
			Build()
		handler := newDeleteHandler(t, []ctrlclient.Object{
			provisionedClusterWithKonnector(),
			konnectorHelmChartProxy(true),
		}, func(context.Context, string, ctrlclient.Client, ctrlclient.ObjectKey) (ctrlclient.Client, error) {
			return remoteClient, nil
		})

		resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
		handler.BeforeClusterDelete(context.Background(), deleteRequest(), resp)

		assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.Status)
		assert.Equal(t, int32(5), resp.RetryAfterSeconds)
		assert.Contains(t, resp.Message, "cleanup in progress")
		assertHelmChartProxyPresent(t, handler.client)
	})

	t.Run("config map gone while helm cleanup is in progress allows deletion", func(t *testing.T) {
		t.Parallel()

		remoteClient := fake.NewClientBuilder().
			WithScheme(deleteTestScheme()).
			Build()
		handler := newDeleteHandler(t, []ctrlclient.Object{
			provisionedClusterWithKonnector(),
			konnectorHelmChartProxy(true),
		}, func(context.Context, string, ctrlclient.Client, ctrlclient.ObjectKey) (ctrlclient.Client, error) {
			return remoteClient, nil
		})

		resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
		handler.BeforeClusterDelete(context.Background(), deleteRequest(), resp)

		assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
		assert.Equal(t, int32(0), resp.RetryAfterSeconds)
		assert.Empty(t, resp.Message)
		assertHelmChartProxyPresent(t, handler.client)
	})
}

func assertHelmChartProxyPresent(t *testing.T, c ctrlclient.Client) {
	t.Helper()

	hcp := &caaphv1.HelmChartProxy{}
	err := c.Get(context.Background(), ctrlclient.ObjectKey{
		Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, deleteTestClusterUUID),
		Namespace: deleteTestNamespace,
	}, hcp)
	require.NoError(t, err)
}
