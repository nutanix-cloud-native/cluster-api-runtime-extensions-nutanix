// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallbconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/workqueue"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	apivariables "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/serviceloadbalancer/metallb"
)

func unitScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(s))
	require.NoError(t, clusterv1.AddToScheme(s))
	require.NoError(t, caaphv1.AddToScheme(s))
	require.NoError(t, metallbv1.AddToScheme(s))
	return s
}

func newClusterVariable(t *testing.T, slb *v1alpha1.ServiceLoadBalancer) clusterv1.ClusterVariable {
	t.Helper()
	cv, err := apivariables.MarshalToClusterVariable(
		v1alpha1.ClusterConfigVariableName,
		&apivariables.ClusterConfigSpec{
			Addons: &apivariables.Addons{
				GenericAddons: v1alpha1.GenericAddons{
					ServiceLoadBalancer: slb,
				},
			},
		},
	)
	require.NoError(t, err)
	return *cv
}

func newTestCluster(
	t *testing.T,
	namespace, name, clusterUUID string,
	slb *v1alpha1.ServiceLoadBalancer,
) *clusterv1.Cluster {
	t.Helper()
	var annotations map[string]string
	if clusterUUID != "" {
		annotations = map[string]string{
			v1alpha1.ClusterUUIDAnnotationKey: clusterUUID,
		}
	}
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "test-class"},
				Version:  "v1.35.0",
				Variables: []clusterv1.ClusterVariable{
					newClusterVariable(t, slb),
				},
			},
		},
	}
}

func newReadyHelmChartProxy(namespace, clusterName, clusterUUID string) *caaphv1.HelmChartProxy {
	return &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       metallb.HelmChartProxyName(clusterUUID),
			Namespace:  namespace,
			Generation: 2,
		},
		Spec: caaphv1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: clusterName},
			},
			ChartName:        metallb.DefaultHelmReleaseName,
			RepoURL:          "https://metallb.github.io/metallb",
			ReleaseName:      metallb.DefaultHelmReleaseName,
			ReleaseNamespace: metallb.DefaultHelmReleaseNamespace,
		},
		Status: caaphv1.HelmChartProxyStatus{
			ObservedGeneration: 2,
			Conditions: []metav1.Condition{
				{
					Type:               clusterv1.ReadyCondition,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             "HelmReleaseReady",
				},
			},
		},
	}
}

func TestOptions_AddFlags(t *testing.T) {
	t.Parallel()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	opts := &Options{}
	opts.AddFlags(fs)
	require.NoError(t, fs.Parse([]string{"--metallb-config-concurrency=5"}))
	assert.Equal(t, 5, opts.Concurrency)
}

func TestReconciler_SetupWithManager_NilManager(t *testing.T) {
	t.Parallel()

	r := &Reconciler{}
	err := r.SetupWithManager(nil, &controller.Options{MaxConcurrentReconciles: 1})
	require.Error(t, err)
}

func TestControllerOptions_CapsRetryBackoff(t *testing.T) {
	t.Parallel()

	opts := controllerOptions(&controller.Options{MaxConcurrentReconciles: 4})
	assert.Equal(t, 4, opts.MaxConcurrentReconciles)
	require.NotNil(t, opts.RateLimiter)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "crs-cluster"},
	}
	var lastDelay time.Duration
	for range 25 {
		lastDelay = opts.RateLimiter.When(req)
		assert.LessOrEqual(t, lastDelay, defaultMaxRetryDelay)
	}
	assert.Equal(t, defaultMaxRetryDelay, lastDelay)

	opts.RateLimiter.Forget(req)
	assert.Less(t, opts.RateLimiter.When(req), defaultMaxRetryDelay)

	customLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
		time.Millisecond,
		2*time.Second,
	)
	preserved := controllerOptions(&controller.Options{RateLimiter: customLimiter})
	assert.Same(t, customLimiter, preserved.RateLimiter)

	nilOpts := controllerOptions(nil)
	assert.NotNil(t, nilOpts.RateLimiter)
}

func TestIsMetalLBHelmChartProxy(t *testing.T) {
	t.Parallel()

	assert.True(t, isMetalLBHelmChartProxy(&caaphv1.HelmChartProxy{
		Spec: caaphv1.HelmChartProxySpec{
			ReleaseName:      metallb.DefaultHelmReleaseName,
			ReleaseNamespace: metallb.DefaultHelmReleaseNamespace,
		},
	}))
	assert.False(t, isMetalLBHelmChartProxy(&caaphv1.HelmChartProxy{
		Spec: caaphv1.HelmChartProxySpec{
			ReleaseName:      "cilium",
			ReleaseNamespace: "kube-system",
		},
	}))
	assert.False(t, isMetalLBHelmChartProxy(&corev1.ConfigMap{}))
}

func TestClusterPredicate(t *testing.T) {
	t.Parallel()

	pred := clusterPredicate()
	base := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "c1",
			Namespace:   "default",
			Generation:  1,
			Annotations: map[string]string{"a": "1"},
		},
	}

	// Status-only update should be ignored.
	statusUpdated := base.DeepCopy()
	statusUpdated.Status.Phase = string(clusterv1.ClusterPhaseProvisioned)
	assert.False(t, pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: statusUpdated}))

	// Generation bump (spec change) should trigger reconciliation.
	genUpdated := base.DeepCopy()
	genUpdated.Generation = 2
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: genUpdated}))

	// Annotation change (e.g. cluster-uuid or paused) should trigger reconciliation.
	annUpdated := base.DeepCopy()
	annUpdated.Annotations["caren.nutanix.com/cluster-uuid"] = "uuid-1"
	assert.True(t, pred.Update(event.UpdateEvent{ObjectOld: base, ObjectNew: annUpdated}))

	// Delete event should be ignored.
	assert.False(t, pred.Delete(event.DeleteEvent{Object: base}))
}

func TestHelmChartProxyToCluster(t *testing.T) {
	t.Parallel()

	t.Run("maps from Cluster OwnerReference even when Controller is nil", func(t *testing.T) {
		t.Parallel()

		hcp := &caaphv1.HelmChartProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "metallb-uuid-1",
				Namespace: "clusters-ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       clusterv1.ClusterKind,
						Name:       "my-cluster",
					},
				},
			},
		}

		reqs := helmChartProxyToCluster(t.Context(), hcp)
		assert.Equal(t, []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: "clusters-ns", Name: "my-cluster"}},
		}, reqs)
	})

	t.Run("falls back to ClusterSelector label when OwnerReference is absent", func(t *testing.T) {
		t.Parallel()

		hcp := &caaphv1.HelmChartProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "metallb-uuid-1",
				Namespace: "clusters-ns",
			},
			Spec: caaphv1.HelmChartProxySpec{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						clusterv1.ClusterNameLabel: "selector-cluster",
					},
				},
			},
		}

		reqs := helmChartProxyToCluster(t.Context(), hcp)
		assert.Equal(t, []reconcile.Request{
			{NamespacedName: types.NamespacedName{Namespace: "clusters-ns", Name: "selector-cluster"}},
		}, reqs)
	})
}

func TestReconciler_Reconcile_Unit(t *testing.T) {
	t.Parallel()

	defaultSLB := &v1alpha1.ServiceLoadBalancer{
		Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
		Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
			AddressRanges: []v1alpha1.AddressRange{
				{Start: "10.100.1.1", End: "10.100.1.20"},
			},
		},
	}

	t.Run("cluster not found returns nil", func(t *testing.T) {
		t.Parallel()

		r := &Reconciler{
			Client: fake.NewClientBuilder().WithScheme(unitScheme(t)).Build(),
		}
		res, err := r.Reconcile(t.Context(), reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: "default", Name: "missing"},
		})
		require.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
	})

	t.Run("skips deleting cluster", func(t *testing.T) {
		t.Parallel()

		cluster := newTestCluster(t, "default", "c1", "uuid-1", defaultSLB)
		now := metav1.Now()
		cluster.DeletionTimestamp = &now
		cluster.Finalizers = []string{"test-finalizer"}

		remoteCalled := false
		r := &Reconciler{
			Client: fake.NewClientBuilder().
				WithScheme(unitScheme(t)).
				WithObjects(cluster, newReadyHelmChartProxy("default", "c1", "uuid-1")).
				Build(),
			RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
				remoteCalled = true
				return nil, nil
			},
		}

		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		require.NoError(t, err)
		assert.False(t, remoteCalled)
	})

	t.Run("skips paused cluster", func(t *testing.T) {
		t.Parallel()

		cluster := newTestCluster(t, "default", "c1", "uuid-1", defaultSLB)
		cluster.Spec.Paused = new(true)

		remoteCalled := false
		r := &Reconciler{
			Client: fake.NewClientBuilder().
				WithScheme(unitScheme(t)).
				WithObjects(cluster, newReadyHelmChartProxy("default", "c1", "uuid-1")).
				Build(),
			RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
				remoteCalled = true
				return nil, nil
			},
		}

		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		require.NoError(t, err)
		assert.False(t, remoteCalled)
	})

	t.Run("skips cluster without topology or without MetalLB configuration", func(t *testing.T) {
		t.Parallel()

		noTopology := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "no-topo", Namespace: "default"},
		}
		noConfig := newTestCluster(t, "default", "no-cfg", "uuid-2", &v1alpha1.ServiceLoadBalancer{
			Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
		})
		noSLB := newTestCluster(t, "default", "no-slb", "uuid-3", nil)

		for _, cl := range []*clusterv1.Cluster{noTopology, noConfig, noSLB} {
			remoteCalled := false
			r := &Reconciler{
				Client: fake.NewClientBuilder().WithScheme(unitScheme(t)).WithObjects(cl).Build(),
				RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
					remoteCalled = true
					return nil, nil
				},
			}
			_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cl)})
			require.NoError(t, err)
			assert.False(t, remoteCalled)
		}
	})

	t.Run("returns error on malformed serviceLoadBalancer variable", func(t *testing.T) {
		t.Parallel()

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "bad-var", Namespace: "default"},
			Spec: clusterv1.ClusterSpec{
				Topology: clusterv1.Topology{
					ClassRef: clusterv1.ClusterClassRef{Name: "test-class"},
					Version:  "v1.35.0",
					Variables: []clusterv1.ClusterVariable{
						{
							Name: v1alpha1.ClusterConfigVariableName,
							Value: apiextensionsv1.JSON{
								Raw: []byte(`{"addons":{"serviceLoadBalancer":{"provider": 123}}}`),
							},
						},
					},
				},
			},
		}

		recorder := events.NewFakeRecorder(10)
		r := &Reconciler{
			Client:   fake.NewClientBuilder().WithScheme(unitScheme(t)).WithObjects(cluster).Build(),
			Recorder: recorder,
		}
		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		require.Error(t, err)
		require.Len(t, recorder.Events, 1)
		recordedEvent := <-recorder.Events
		assert.Contains(t, recordedEvent, corev1.EventTypeWarning+" "+MetalLBConfigurationFailedReason+" ")
		assert.Contains(t, recordedEvent, "failed to read ServiceLoadBalancer configuration")
	})

	t.Run("waits without error when UUID annotation or HelmChartProxy is missing or unready", func(t *testing.T) {
		t.Parallel()

		noUUID := newTestCluster(t, "default", "no-uuid", "", defaultSLB)
		missingHCP := newTestCluster(t, "default", "missing-hcp", "uuid-missing", defaultSLB)
		unreadyCluster := newTestCluster(t, "default", "unready-hcp", "uuid-unready", defaultSLB)
		unreadyHCP := newReadyHelmChartProxy("default", "unready-hcp", "uuid-unready")
		unreadyHCP.Status.ObservedGeneration = 1 // Generation is 2.

		falseReadyCluster := newTestCluster(t, "default", "false-ready", "uuid-false", defaultSLB)
		falseReadyHCP := newReadyHelmChartProxy("default", "false-ready", "uuid-false")
		falseReadyHCP.Status.Conditions[0].Status = metav1.ConditionFalse

		remoteCalled := false
		r := &Reconciler{
			Client: fake.NewClientBuilder().
				WithScheme(unitScheme(t)).
				WithObjects(noUUID, missingHCP, unreadyCluster, unreadyHCP, falseReadyCluster, falseReadyHCP).
				Build(),
			RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
				remoteCalled = true
				return nil, nil
			},
		}

		for _, cl := range []*clusterv1.Cluster{noUUID, missingHCP, unreadyCluster, falseReadyCluster} {
			res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cl)})
			require.NoError(t, err)
			assert.Equal(t, reconcile.Result{}, res)
			assert.False(t, remoteCalled)
		}
	})

	t.Run("applies MetalLB configuration when HelmChartProxy is Ready", func(t *testing.T) {
		t.Parallel()

		scheme := unitScheme(t)
		cluster := newTestCluster(t, "default", "ready-cluster", "uuid-ready", defaultSLB)
		hcp := newReadyHelmChartProxy("default", "ready-cluster", "uuid-ready")
		remoteClient := fake.NewClientBuilder().WithScheme(scheme).Build()

		r := &Reconciler{
			Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, hcp).Build(),
			RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
				return remoteClient, nil
			},
		}

		res, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		require.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)

		var pool metallbv1.IPAddressPool
		require.NoError(t, remoteClient.Get(
			t.Context(),
			client.ObjectKey{Namespace: metallb.DefaultHelmReleaseNamespace, Name: metallb.DefaultHelmReleaseName},
			&pool,
		))
		assert.Equal(t, []string{"10.100.1.1-10.100.1.20"}, pool.Spec.Addresses)

		var l2 metallbv1.L2Advertisement
		require.NoError(t, remoteClient.Get(
			t.Context(),
			client.ObjectKey{Namespace: metallb.DefaultHelmReleaseNamespace, Name: metallb.DefaultHelmReleaseName},
			&l2,
		))
		assert.Equal(t, []string{metallb.DefaultHelmReleaseName}, l2.Spec.IPAddressPools)
	})

	t.Run("returns conflict error when workload cluster resource was modified", func(t *testing.T) {
		t.Parallel()

		scheme := unitScheme(t)
		cluster := newTestCluster(t, "default", "conflict-cluster", "uuid-conflict", defaultSLB)
		hcp := newReadyHelmChartProxy("default", "conflict-cluster", "uuid-conflict")
		remoteClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(
					_ context.Context,
					_ client.WithWatch,
					obj client.Object,
					_ client.Patch,
					_ ...client.PatchOption,
				) error {
					return apierrors.NewConflict(
						schema.GroupResource{Group: metallbv1.GroupVersion.Group, Resource: "ipaddresspools"},
						obj.GetName(),
						errors.New("conflict"),
					)
				},
			}).
			Build()

		recorder := events.NewFakeRecorder(10)
		r := &Reconciler{
			Client:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, hcp).Build(),
			Recorder: recorder,
			RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
				return remoteClient, nil
			},
		}

		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		require.Error(t, err)
		assert.ErrorContains(
			t,
			err,
			"This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration",
		)
		require.Len(t, recorder.Events, 1)
		recordedEvent := <-recorder.Events
		assert.Contains(t, recordedEvent, corev1.EventTypeWarning+" "+MetalLBConfigurationFailedReason+" ")
		assert.Contains(
			t,
			recordedEvent,
			"This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration",
		)
	})

	t.Run("truncates overlong Warning event notes safely without format-string corruption", func(t *testing.T) {
		t.Parallel()

		scheme := unitScheme(t)
		cluster := newTestCluster(t, "default", "long-err-cluster", "uuid-long", defaultSLB)
		hcp := newReadyHelmChartProxy("default", "long-err-cluster", "uuid-long")
		recorder := events.NewFakeRecorder(10)

		r := &Reconciler{
			Client:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, hcp).Build(),
			Recorder: recorder,
			RemoteClientGetter: func(_ context.Context, _ client.Client, _ client.ObjectKey) (client.Client, error) {
				return nil, fmt.Errorf("webhook %s failed: %s", "%2F%s%d", strings.Repeat("é", 700))
			},
		}

		_, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
		require.Error(t, err)
		require.Len(t, recorder.Events, 1)
		recordedEvent := <-recorder.Events
		prefix := corev1.EventTypeWarning + " " + MetalLBConfigurationFailedReason + " "
		require.True(t, strings.HasPrefix(recordedEvent, prefix))
		note := strings.TrimPrefix(recordedEvent, prefix)
		assert.LessOrEqual(t, len(note), maxEventNoteLength)
		assert.True(t, utf8.ValidString(note))
		assert.Contains(t, note, "%2F%s%d")
		assert.NotContains(t, note, "%!(EXTRA")
		assert.NotContains(t, note, "%!(MISSING")
		assert.True(t, strings.HasSuffix(note, "..."))
	})
}

func TestIntegrationReconcileMetalLBConfiguration(t *testing.T) {
	g := NewWithT(t)

	ns, err := env.CreateNamespace(ctx, "metallb-cfg", nil)
	g.Expect(err).NotTo(HaveOccurred())

	// Ensure metallb-system namespace exists in the test API server (which acts as the workload cluster).
	metallbNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: metallb.DefaultHelmReleaseNamespace,
		},
	}
	err = env.Client.Create(ctx, metallbNS)
	if !apierrors.IsAlreadyExists(err) {
		g.Expect(err).NotTo(HaveOccurred())
	}

	clusterUUID := "integration-uuid-1"
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-metallb-cluster",
			Namespace: ns.Name,
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: clusterUUID,
			},
		},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "test-class"},
				Version:  "v1.35.0",
				Variables: []clusterv1.ClusterVariable{
					newClusterVariable(t, &v1alpha1.ServiceLoadBalancer{
						Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
						Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
							AddressRanges: []v1alpha1.AddressRange{
								{Start: "10.100.1.1", End: "10.100.1.20"},
							},
						},
					}),
				},
			},
		},
	}
	g.Expect(env.CreateAndWait(ctx, cluster)).To(Succeed())
	g.Expect(env.CreateKubeconfigSecret(ctx, cluster)).To(Succeed())

	poolKey := client.ObjectKey{
		Namespace: metallb.DefaultHelmReleaseNamespace,
		Name:      metallb.DefaultHelmReleaseName,
	}
	defer func() {
		_ = env.Cleanup(
			ctx,
			&metallbv1.IPAddressPool{ObjectMeta: metav1.ObjectMeta{Namespace: poolKey.Namespace, Name: poolKey.Name}},
			&metallbv1.L2Advertisement{ObjectMeta: metav1.ObjectMeta{Namespace: poolKey.Namespace, Name: poolKey.Name}},
			cluster,
		)
	}()

	// 1. Before HelmChartProxy exists, configuration resources must not be created.
	g.Consistently(func() bool {
		err := env.GetAPIReader().Get(ctx, poolKey, &metallbv1.IPAddressPool{})
		return apierrors.IsNotFound(err)
	}, 1*time.Second, 100*time.Millisecond).Should(BeTrue())

	// 2. Create HelmChartProxy with non-controller OwnerReference (matching helmAddonApplier.Apply),
	//    but without Ready=True status yet.
	hcp := &caaphv1.HelmChartProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metallb.HelmChartProxyName(clusterUUID),
			Namespace: ns.Name,
		},
		Spec: caaphv1.HelmChartProxySpec{
			ClusterSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
			},
			ChartName:        metallb.DefaultHelmReleaseName,
			RepoURL:          "https://metallb.github.io/metallb",
			ReleaseName:      metallb.DefaultHelmReleaseName,
			ReleaseNamespace: metallb.DefaultHelmReleaseNamespace,
		},
	}
	g.Expect(controllerutil.SetOwnerReference(cluster, hcp, env.GetScheme())).To(Succeed())
	g.Expect(env.CreateAndWait(ctx, hcp)).To(Succeed())
	defer func() {
		_ = env.Cleanup(ctx, hcp)
	}()

	g.Consistently(func() bool {
		err := env.GetAPIReader().Get(ctx, poolKey, &metallbv1.IPAddressPool{})
		return apierrors.IsNotFound(err)
	}, 1*time.Second, 100*time.Millisecond).Should(BeTrue())

	// 3. Mark HelmChartProxy Ready=True; watch must enqueue the Cluster and apply IPAddressPool & L2Advertisement.
	g.Eventually(func() error {
		var latest caaphv1.HelmChartProxy
		if err := env.GetAPIReader().Get(ctx, client.ObjectKeyFromObject(hcp), &latest); err != nil {
			return err
		}
		latest.Status.ObservedGeneration = latest.Generation
		latest.Status.MatchingClusters = []corev1.ObjectReference{
			{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       clusterv1.ClusterKind,
				Name:       cluster.Name,
				Namespace:  cluster.Namespace,
			},
		}
		latest.Status.Conditions = []metav1.Condition{
			{
				Type:               clusterv1.ReadyCondition,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "HelmReleaseReady",
			},
		}
		return env.Client.Status().Update(ctx, &latest)
	}).Should(Succeed())

	g.Eventually(func() ([]string, error) {
		var pool metallbv1.IPAddressPool
		if err := env.GetAPIReader().Get(ctx, poolKey, &pool); err != nil {
			return nil, err
		}
		return pool.Spec.Addresses, nil
	}).Should(Equal([]string{"10.100.1.1-10.100.1.20"}))

	g.Eventually(func() ([]string, error) {
		var l2 metallbv1.L2Advertisement
		if err := env.GetAPIReader().Get(ctx, poolKey, &l2); err != nil {
			return nil, err
		}
		return l2.Spec.IPAddressPools, nil
	}).Should(Equal([]string{metallb.DefaultHelmReleaseName}))

	// 4. Update Cluster addressRanges; Cluster watch must update IPAddressPool automatically.
	g.Eventually(func() error {
		var latestCluster clusterv1.Cluster
		if err := env.GetAPIReader().Get(ctx, client.ObjectKeyFromObject(cluster), &latestCluster); err != nil {
			return err
		}
		latestCluster.Spec.Topology.Variables = []clusterv1.ClusterVariable{
			newClusterVariable(t, &v1alpha1.ServiceLoadBalancer{
				Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
				Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
					AddressRanges: []v1alpha1.AddressRange{
						{Start: "10.100.1.1", End: "10.100.1.20"},
						{Start: "10.100.1.51", End: "10.100.1.70"},
					},
				},
			}),
		}
		return env.Client.Update(ctx, &latestCluster)
	}).Should(Succeed())

	g.Eventually(func() ([]string, error) {
		var pool metallbv1.IPAddressPool
		if err := env.GetAPIReader().Get(ctx, poolKey, &pool); err != nil {
			return nil, err
		}
		return pool.Spec.Addresses, nil
	}).Should(Equal([]string{"10.100.1.1-10.100.1.20", "10.100.1.51-10.100.1.70"}))

	// 5. Modify IPAddressPool on the workload cluster with a conflicting field manager;
	//    direct reconciliation against real kube-apiserver SSA must return the expected conflict message.
	conflictingPool := &metallbv1.IPAddressPool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: metallbv1.GroupVersion.String(),
			Kind:       "IPAddressPool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      poolKey.Name,
			Namespace: poolKey.Namespace,
		},
		Spec: metallbv1.IPAddressPoolSpec{
			Addresses: []string{"192.168.10.1-192.168.10.10"},
		},
	}
	g.Expect(
		env.Client.Patch(
			ctx,
			conflictingPool,
			client.Apply,
			client.FieldOwner("manual-operator"),
			client.ForceOwnership,
		),
	).To(Succeed())

	reconciler := &Reconciler{Client: env.Client}
	_, err = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(
		fmt.Sprintf("failed to apply MetalLB configuration IPAddressPool %s", poolKey),
	))
	g.Expect(err.Error()).To(ContainSubstring(
		"This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration",
	))

	// Trigger the manager-registered controller and verify a real events.k8s.io/v1 Warning Event
	// is recorded against the Cluster (with the MetalLB HelmChartProxy as Related).
	g.Eventually(func() error {
		var latestCluster clusterv1.Cluster
		if err := env.GetAPIReader().Get(ctx, client.ObjectKeyFromObject(cluster), &latestCluster); err != nil {
			return err
		}
		if latestCluster.Annotations == nil {
			latestCluster.Annotations = map[string]string{}
		}
		latestCluster.Annotations["caren.nutanix.com/reconcile-trigger"] = "conflict-check"
		return env.Client.Update(ctx, &latestCluster)
	}).Should(Succeed())

	g.Eventually(func() bool {
		var eventList eventsv1.EventList
		if err := env.GetAPIReader().List(ctx, &eventList, client.InNamespace(ns.Name)); err != nil {
			return false
		}
		for _, evt := range eventList.Items {
			if evt.Regarding.Kind == clusterv1.ClusterKind &&
				evt.Regarding.Name == cluster.Name &&
				evt.Type == corev1.EventTypeWarning &&
				evt.Reason == MetalLBConfigurationFailedReason &&
				evt.Action == ReconcileMetalLBConfigurationAction &&
				evt.ReportingController == ControllerName &&
				evt.Related != nil &&
				evt.Related.Kind == "HelmChartProxy" &&
				evt.Related.Name == hcp.Name &&
				strings.Contains(
					evt.Note,
					"This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration",
				) {
				return true
			}
		}
		return false
	}).Should(BeTrue())
}
