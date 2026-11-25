//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capie2e "sigs.k8s.io/cluster-api/test/e2e"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/test/e2e/framework"
)

type stubClusterProxy struct {
	capiframework.ClusterProxy
	name           string
	kubeconfigPath string
	k8sClient      client.Client
	logsCollected  bool
}

func (p *stubClusterProxy) GetName() string {
	return p.name
}

func (p *stubClusterProxy) GetKubeconfigPath() string {
	return p.kubeconfigPath
}

func (p *stubClusterProxy) GetClient() client.Client {
	return p.k8sClient
}

func (p *stubClusterProxy) CollectWorkloadClusterLogs(_ context.Context, _, _, _ string) {
	p.logsCollected = true
}

func newStubClusterProxy(t *testing.T, initObjs ...client.Object) *stubClusterProxy {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	return &stubClusterProxy{
		name:           "bootstrap",
		kubeconfigPath: filepath.Join(t.TempDir(), "kubeconfig"),
		k8sClient:      fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build(),
	}
}

func TestCleanupQuickStartSpec_BeforeEachFailedBeforeSetup(t *testing.T) {
	gomega.RegisterTestingT(t)

	var watchCanceled bool
	require.NotPanics(t, func() {
		framework.CleanupQuickStartSpec(
			context.Background(),
			"quick-start",
			&capie2e.QuickStartSpecInput{},
			nil,
			nil,
			func() { watchCanceled = true },
			nil,
		)
	})
	assert.True(t, watchCanceled, "cancelWatches should still run when non-nil")
}

func TestCleanupQuickStartSpec_NamespaceCreatedBeforeClusterInitialized(t *testing.T) {
	gomega.RegisterTestingT(t)

	for _, tc := range []struct {
		name             string
		clusterResources *clusterctl.ApplyClusterTemplateAndWaitResult
	}{
		{
			name:             "nil clusterResources",
			clusterResources: nil,
		},
		{
			name:             "empty clusterResources with nil Cluster",
			clusterResources: &clusterctl.ApplyClusterTemplateAndWaitResult{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gomega.RegisterTestingT(t)

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "quick-start-abc123"}}
			ccNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: "quick-start-abc123-clusterclass"},
			}
			proxy := newStubClusterProxy(t, ns, ccNs)
			var watchCanceled bool

			input := &capie2e.QuickStartSpecInput{
				E2EConfig: &clusterctl.E2EConfig{
					Intervals: map[string][]string{
						"default/wait-delete-cluster": {"1s", "100ms"},
					},
				},
				ClusterctlConfigPath:                  filepath.Join(t.TempDir(), "clusterctl.yaml"),
				BootstrapClusterProxy:                 proxy,
				ArtifactFolder:                        t.TempDir(),
				DeployClusterClassInSeparateNamespace: true,
			}

			require.NotPanics(t, func() {
				framework.CleanupQuickStartSpec(
					context.Background(),
					"quick-start",
					input,
					ns,
					ccNs,
					func() { watchCanceled = true },
					tc.clusterResources,
				)
			})

			assert.True(t, watchCanceled)
			assert.False(
				t,
				proxy.logsCollected,
				"workload cluster logs must not be collected when cluster was never created",
			)

			err := proxy.GetClient().Get(
				context.Background(),
				client.ObjectKeyFromObject(ns),
				&corev1.Namespace{},
			)
			assert.True(t, apierrors.IsNotFound(err), "spec namespace should be deleted")

			err = proxy.GetClient().Get(
				context.Background(),
				client.ObjectKeyFromObject(ccNs),
				&corev1.Namespace{},
			)
			assert.True(t, apierrors.IsNotFound(err), "clusterclass namespace should be deleted")
		})
	}
}

func TestCleanupQuickStartSpec_SkipCleanupPreservesNamespaces(t *testing.T) {
	gomega.RegisterTestingT(t)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "quick-start-keep"}}
	ccNs := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "quick-start-keep-clusterclass"}}
	proxy := newStubClusterProxy(t, ns, ccNs)
	var watchCanceled bool

	input := &capie2e.QuickStartSpecInput{
		E2EConfig:                             &clusterctl.E2EConfig{},
		ClusterctlConfigPath:                  filepath.Join(t.TempDir(), "clusterctl.yaml"),
		BootstrapClusterProxy:                 proxy,
		ArtifactFolder:                        t.TempDir(),
		SkipCleanup:                           true,
		DeployClusterClassInSeparateNamespace: true,
	}

	require.NotPanics(t, func() {
		framework.CleanupQuickStartSpec(
			context.Background(),
			"quick-start",
			input,
			ns,
			ccNs,
			func() { watchCanceled = true },
			&clusterctl.ApplyClusterTemplateAndWaitResult{},
		)
	})

	assert.True(t, watchCanceled)
	require.NoError(
		t,
		proxy.GetClient().Get(context.Background(), client.ObjectKeyFromObject(ns), &corev1.Namespace{}),
	)
	require.NoError(
		t,
		proxy.GetClient().Get(context.Background(), client.ObjectKeyFromObject(ccNs), &corev1.Namespace{}),
	)
}

func TestCleanupSelfHostedSpec_BeforeEachFailedOrSkippedBeforeSetup(t *testing.T) {
	gomega.RegisterTestingT(t)

	var (
		bootstrapWatchCanceled  bool
		selfHostedWatchCanceled bool
	)
	require.NotPanics(t, func() {
		framework.CleanupSelfHostedSpec(
			context.Background(),
			"self-hosted",
			&framework.SelfHostedSpecInput{},
			nil,
			func() { bootstrapWatchCanceled = true },
			nil,
			nil,
			nil,
			func() { selfHostedWatchCanceled = true },
			nil,
		)
	})
	assert.True(t, bootstrapWatchCanceled)
	assert.True(t, selfHostedWatchCanceled)
}

func TestCleanupSelfHostedSpec_NamespaceCreatedBeforeClusterInitialized(t *testing.T) {
	gomega.RegisterTestingT(t)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "self-hosted-abc123"}}
	proxy := newStubClusterProxy(t, ns)
	var watchCanceled bool

	input := &framework.SelfHostedSpecInput{
		E2EConfig: &clusterctl.E2EConfig{
			Intervals: map[string][]string{
				"default/wait-delete-cluster": {"1s", "100ms"},
			},
		},
		ClusterctlConfigPath:  filepath.Join(t.TempDir(), "clusterctl.yaml"),
		BootstrapClusterProxy: proxy,
		ArtifactFolder:        t.TempDir(),
	}

	require.NotPanics(t, func() {
		framework.CleanupSelfHostedSpec(
			context.Background(),
			"self-hosted",
			input,
			ns,
			func() { watchCanceled = true },
			nil,
			nil,
			nil,
			nil,
			nil,
		)
	})

	assert.True(t, watchCanceled)
	assert.False(t, proxy.logsCollected)
	err := proxy.GetClient().Get(
		context.Background(),
		client.ObjectKeyFromObject(ns),
		&corev1.Namespace{},
	)
	assert.True(t, apierrors.IsNotFound(err), "spec namespace should be deleted")
}

func TestCleanupQuickStartSpec_PopulatedClusterCollectsLogsAndCleansUp(t *testing.T) {
	gomega.RegisterTestingT(t)

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "quick-start-populated"}}
	proxy := newStubClusterProxy(t, ns)
	var watchCanceled bool

	input := &capie2e.QuickStartSpecInput{
		E2EConfig: &clusterctl.E2EConfig{
			Intervals: map[string][]string{
				"default/wait-delete-cluster": {"1s", "100ms"},
			},
		},
		ClusterctlConfigPath:  filepath.Join(t.TempDir(), "clusterctl.yaml"),
		BootstrapClusterProxy: proxy,
		ArtifactFolder:        t.TempDir(),
	}

	clusterResources := &clusterctl.ApplyClusterTemplateAndWaitResult{
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "workload-1",
				Namespace: ns.Name,
			},
		},
	}

	require.NotPanics(t, func() {
		framework.CleanupQuickStartSpec(
			context.Background(),
			"quick-start",
			input,
			ns,
			nil,
			func() { watchCanceled = true },
			clusterResources,
		)
	})

	assert.True(t, watchCanceled)
	assert.True(t, proxy.logsCollected, "workload cluster logs should be collected when cluster is set")
	err := proxy.GetClient().Get(
		context.Background(),
		client.ObjectKeyFromObject(ns),
		&corev1.Namespace{},
	)
	assert.True(t, apierrors.IsNotFound(err), "spec namespace should be deleted")
}
