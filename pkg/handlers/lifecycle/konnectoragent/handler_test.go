// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package konnectoragent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

const (
	testClusterName      = "quick-start-cluster"
	testClusterNamespace = "quick-start-ns"
	testClusterUUID      = "aaaa1111-2222-3333-4444-555566667777"
	testKubeSystemUID    = "11111111-2222-3333-4444-555566667777"
	testCredsSecretName  = "pc-creds-for-konnector-agent"
)

func newBCDTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(s))
	utilruntime.Must(clusterv1.AddToScheme(s))
	utilruntime.Must(caaphv1.AddToScheme(s))
	return s
}

func newMockWorkloadAPIServer(t *testing.T, delay time.Duration, calls *atomic.Int32) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls != nil {
			calls.Add(1)
		}
		if delay > 0 && r.URL.Path == "/api/v1/namespaces/kube-system" {
			select {
			case <-time.After(delay):
			case <-r.Context().Done():
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api":
			_, _ = w.Write([]byte(`{"kind":"APIVersions","versions":["v1"]}`))
		case "/apis":
			_, _ = w.Write([]byte(`{"kind":"APIGroupList","apiVersion":"v1","groups":[]}`))
		case "/api/v1":
			_, _ = w.Write([]byte(`{
				"kind":"APIResourceList",
				"groupVersion":"v1",
				"resources":[{"name":"namespaces","singularName":"namespace","namespaced":false,"kind":"Namespace","verbs":["get","list"]}]
			}`))
		case "/api/v1/namespaces/kube-system":
			_, _ = fmt.Fprintf(
				w,
				`{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"kube-system","uid":%q}}`,
				testKubeSystemUID,
			)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type mockPCConfig struct {
	registered   bool
	delay        time.Duration
	getCalls     *atomic.Int32
	deleteCalls  *atomic.Int32
	deleteForced *atomic.Bool
}

func newMockPrismCentralServer(t *testing.T, cfg mockPCConfig) *httptest.Server {
	t.Helper()

	regPath := "/karbon/v1-alpha.1/k8s/cluster-registrations/" + testKubeSystemUID
	kubeconfigPath := regPath + "/kubeconfig"

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.delay > 0 {
			select {
			case <-time.After(cfg.delay):
			case <-r.Context().Done():
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == regPath:
			if cfg.getCalls != nil {
				cfg.getCalls.Add(1)
			}
			if !cfg.registered {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"cluster registration not found"}`))
				return
			}
			_, _ = fmt.Fprintf(
				w,
				`{"name":%q,"uuid":%q,"status":"Registered"}`,
				testClusterName,
				testKubeSystemUID,
			)
		case r.Method == http.MethodDelete && r.URL.Path == kubeconfigPath:
			_, _ = w.Write([]byte(`{"task_uuid":"task-kc-delete"}`))
		case r.Method == http.MethodDelete && r.URL.Path == regPath:
			if cfg.deleteCalls != nil {
				cfg.deleteCalls.Add(1)
			}
			if cfg.deleteForced != nil && r.URL.Query().Get("force") == "true" {
				cfg.deleteForced.Store(true)
			}
			_, _ = fmt.Fprintf(
				w,
				`{"cluster_name":%q,"cluster_uuid":%q,"task_uuid":"task-reg-delete"}`,
				testClusterName,
				testKubeSystemUID,
			)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newKubeconfigSecret(t *testing.T, serverURL string) *corev1.Secret {
	t.Helper()

	kubeconfigYAML := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
  name: test
contexts:
- context:
    cluster: test
    user: test
  name: test
current-context: test
users:
- name: test
  user:
    token: dummy-token
`, serverURL)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName + "-kubeconfig",
			Namespace: testClusterNamespace,
		},
		Data: map[string][]byte{
			"value": []byte(kubeconfigYAML),
		},
	}
}

func newPCCredentialsSecret(t *testing.T) *corev1.Secret {
	t.Helper()

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCredsSecretName,
			Namespace: testClusterNamespace,
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret"),
		},
	}
}

func newDeletingCluster(
	t *testing.T,
	pcURL string,
	phase clusterv1.ClusterPhase,
	cpInitialized bool,
) *clusterv1.Cluster {
	t.Helper()

	condStatus := metav1.ConditionFalse
	if cpInitialized {
		condStatus = metav1.ConditionTrue
	}

	rawVars := fmt.Sprintf(`{
		"nutanix": {
			"prismCentralEndpoint": {
				"url": %q,
				"insecure": true
			}
		},
		"addons": {
			"konnectorAgent": {
				"credentials": {
					"secretRef": {
						"name": %q
					}
				}
			}
		}
	}`, pcURL, testCredsSecretName)

	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testClusterName,
			Namespace: testClusterNamespace,
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: testClusterUUID,
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "nutanix-quick-start"},
				Variables: []clusterv1.ClusterVariable{{
					Name:  v1alpha1.ClusterConfigVariableName,
					Value: apiextensionsv1.JSON{Raw: []byte(rawVars)},
				}},
			},
		},
		Status: clusterv1.ClusterStatus{
			Phase: string(phase),
			Conditions: []metav1.Condition{{
				Type:   clusterv1.ClusterControlPlaneInitializedCondition,
				Status: condStatus,
			}},
		},
	}
}

func newHelmChartProxy(t *testing.T, deletionTimestamp *metav1.Time) *caaphv1.HelmChartProxy {
	t.Helper()

	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", defaultHelmReleaseName, testClusterUUID),
			Namespace: testClusterNamespace,
		},
	}
	if deletionTimestamp != nil {
		hcp.DeletionTimestamp = deletionTimestamp
		hcp.Finalizers = []string{"helmchartproxy.addons.cluster.x-k8s.io"}
	}
	return hcp
}

func newBCDRequest(cluster *clusterv1.Cluster) *runtimehooksv1.BeforeClusterDeleteRequest {
	// CAPI strips Status from the Cluster embedded in hook requests.
	reqCluster := cluster.DeepCopy()
	reqCluster.Status = clusterv1.ClusterStatus{}
	return &runtimehooksv1.BeforeClusterDeleteRequest{Cluster: *reqCluster}
}

func TestBeforeClusterDelete_HelmChartProxyMissingSkipsRemoteCalls(t *testing.T) {
	t.Parallel()

	var workloadAPICalls atomic.Int32
	var pcCalls atomic.Int32
	workloadSrv := newMockWorkloadAPIServer(t, 8*time.Second, &workloadAPICalls)
	pcSrv := newMockPrismCentralServer(t, mockPCConfig{registered: true, getCalls: &pcCalls})

	cluster := newDeletingCluster(t, pcSrv.URL, clusterv1.ClusterPhaseDeleting, true)
	mgmtClient := fake.NewClientBuilder().
		WithScheme(newBCDTestScheme(t)).
		WithObjects(cluster, newKubeconfigSecret(t, workloadSrv.URL), newPCCredentialsSecret(t)).
		Build()

	handler := New(mgmtClient, NewConfig(&options.GlobalOptions{}), &config.HelmChartGetter{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(ctx, newBCDRequest(cluster), resp)

	require.NoError(t, ctx.Err(), "handler should not hit hung workload API when HelmChartProxy does not exist")
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
	assert.Equal(t, int32(0), workloadAPICalls.Load(), "workload API must not be called when HelmChartProxy is absent")
	assert.Equal(t, int32(0), pcCalls.Load(), "Prism Central must not be called when HelmChartProxy is absent")
}

func TestBeforeClusterDelete_InitiatesCleanupWithBlockingSuccessRetry(t *testing.T) {
	t.Parallel()

	workloadSrv := newMockWorkloadAPIServer(t, 0, nil)
	pcSrv := newMockPrismCentralServer(t, mockPCConfig{registered: true})

	cluster := newDeletingCluster(t, pcSrv.URL, clusterv1.ClusterPhaseDeleting, true)
	hcp := newHelmChartProxy(t, nil)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(newBCDTestScheme(t)).
		WithObjects(cluster, hcp, newKubeconfigSecret(t, workloadSrv.URL), newPCCredentialsSecret(t)).
		Build()

	handler := New(mgmtClient, NewConfig(&options.GlobalOptions{}), &config.HelmChartGetter{})

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), newBCDRequest(cluster), resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(5), resp.RetryAfterSeconds)

	gotHCP := &caaphv1.HelmChartProxy{}
	err := mgmtClient.Get(context.Background(), ctrlclient.ObjectKeyFromObject(hcp), gotHCP)
	assert.Error(t, err, "HelmChartProxy should have been deleted")
}

func TestBeforeClusterDelete_CleanupInProgressReturnsBlockingSuccessRetry(t *testing.T) {
	t.Parallel()

	workloadSrv := newMockWorkloadAPIServer(t, 0, nil)
	pcSrv := newMockPrismCentralServer(t, mockPCConfig{registered: true})

	cluster := newDeletingCluster(t, pcSrv.URL, clusterv1.ClusterPhaseDeleting, true)
	deletingSince := metav1.NewTime(time.Now().Add(-30 * time.Second))
	hcp := newHelmChartProxy(t, &deletingSince)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(newBCDTestScheme(t)).
		WithObjects(cluster, hcp, newKubeconfigSecret(t, workloadSrv.URL), newPCCredentialsSecret(t)).
		Build()

	handler := New(mgmtClient, NewConfig(&options.GlobalOptions{}), &config.HelmChartGetter{})

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), newBCDRequest(cluster), resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(5), resp.RetryAfterSeconds)
}

func TestBeforeClusterDelete_CleanupInProgressButAlreadyUnregisteredAllowsDeletion(t *testing.T) {
	t.Parallel()

	workloadSrv := newMockWorkloadAPIServer(t, 0, nil)
	pcSrv := newMockPrismCentralServer(t, mockPCConfig{registered: false})

	cluster := newDeletingCluster(t, pcSrv.URL, clusterv1.ClusterPhaseDeleting, true)
	deletingSince := metav1.NewTime(time.Now().Add(-30 * time.Second))
	hcp := newHelmChartProxy(t, &deletingSince)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(newBCDTestScheme(t)).
		WithObjects(cluster, hcp, newKubeconfigSecret(t, workloadSrv.URL), newPCCredentialsSecret(t)).
		Build()

	handler := New(mgmtClient, NewConfig(&options.GlobalOptions{}), &config.HelmChartGetter{})

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), newBCDRequest(cluster), resp)

	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
}

func TestBeforeClusterDelete_CleanupTimedOutAllowsClusterDeletionAndForceCleansPC(t *testing.T) {
	t.Parallel()

	var deleteCalls atomic.Int32
	var deleteForced atomic.Bool
	workloadSrv := newMockWorkloadAPIServer(t, 0, nil)
	pcSrv := newMockPrismCentralServer(t, mockPCConfig{
		registered:   true,
		deleteCalls:  &deleteCalls,
		deleteForced: &deleteForced,
	})

	cluster := newDeletingCluster(t, pcSrv.URL, clusterv1.ClusterPhaseDeleting, true)
	timedOutSince := metav1.NewTime(time.Now().Add(-(helmUninstallTimeout + time.Minute)))
	hcp := newHelmChartProxy(t, &timedOutSince)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(newBCDTestScheme(t)).
		WithObjects(cluster, hcp, newKubeconfigSecret(t, workloadSrv.URL), newPCCredentialsSecret(t)).
		Build()

	handler := New(mgmtClient, NewConfig(&options.GlobalOptions{}), &config.HelmChartGetter{})

	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(context.Background(), newBCDRequest(cluster), resp)

	// After helmUninstallTimeout (5m), the handler must allow cluster deletion to proceed
	// instead of blocking BeforeClusterDelete indefinitely.
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
	assert.GreaterOrEqual(t, deleteCalls.Load(), int32(1), "should attempt best-effort forced PC deregistration")
	assert.True(t, deleteForced.Load(), "best-effort PC deregistration should set force=true")
}

func TestBeforeClusterDelete_BoundsHungPrismCentralCall(t *testing.T) {
	t.Parallel()

	workloadSrv := newMockWorkloadAPIServer(t, 0, nil)
	// Prism Central hangs longer than remoteCallTimeout (5s).
	pcSrv := newMockPrismCentralServer(t, mockPCConfig{registered: true, delay: 12 * time.Second})

	cluster := newDeletingCluster(t, pcSrv.URL, clusterv1.ClusterPhaseDeleting, true)
	hcp := newHelmChartProxy(t, nil)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(newBCDTestScheme(t)).
		WithObjects(cluster, hcp, newKubeconfigSecret(t, workloadSrv.URL), newPCCredentialsSecret(t)).
		Build()

	handler := New(mgmtClient, NewConfig(&options.GlobalOptions{}), &config.HelmChartGetter{})

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	start := time.Now()
	resp := &runtimehooksv1.BeforeClusterDeleteResponse{}
	handler.BeforeClusterDelete(ctx, newBCDRequest(cluster), resp)
	elapsed := time.Since(start)

	require.NoError(t, ctx.Err(), "handler must bound hung Prism Central calls before outer context expires")
	assert.Less(t, elapsed, 7*time.Second)
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.Status)
	assert.Equal(t, int32(0), resp.RetryAfterSeconds)
}
