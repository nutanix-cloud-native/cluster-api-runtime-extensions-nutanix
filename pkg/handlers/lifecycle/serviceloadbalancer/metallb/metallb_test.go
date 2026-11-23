// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package metallb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	metallbv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/go.universe.tf/metallb/api/v1beta1"
	caaphv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-addon-provider-helm/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/config"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/options"
)

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, caaphv1.AddToScheme(scheme))
	require.NoError(t, metallbv1.AddToScheme(scheme))
	return scheme
}

func TestHelmChartProxyName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "metallb-cluster-uuid-123", HelmChartProxyName("cluster-uuid-123"))
}

func TestConfigurationObjects(t *testing.T) {
	t.Parallel()

	t.Run("returns error when addressRanges is empty", func(t *testing.T) {
		t.Parallel()

		objs, err := ConfigurationObjects(&ConfigurationInput{
			Name:      DefaultHelmReleaseName,
			Namespace: DefaultHelmReleaseNamespace,
		})
		require.ErrorContains(t, err, "must define one or more AddressRanges")
		assert.Nil(t, objs)
	})

	t.Run("builds IPAddressPool and L2Advertisement", func(t *testing.T) {
		t.Parallel()

		objs, err := ConfigurationObjects(&ConfigurationInput{
			Name:      DefaultHelmReleaseName,
			Namespace: DefaultHelmReleaseNamespace,
			AddressRanges: []v1alpha1.AddressRange{
				{Start: "10.100.1.1", End: "10.100.1.20"},
				{Start: "10.100.1.51", End: "10.100.1.70"},
			},
		})
		require.NoError(t, err)
		require.Len(t, objs, 2)

		pool, ok := objs[0].(*metallbv1.IPAddressPool)
		require.True(t, ok)
		assert.Equal(t, DefaultHelmReleaseName, pool.Name)
		assert.Equal(t, DefaultHelmReleaseNamespace, pool.Namespace)
		assert.Equal(t, []string{"10.100.1.1-10.100.1.20", "10.100.1.51-10.100.1.70"}, pool.Spec.Addresses)

		l2, ok := objs[1].(*metallbv1.L2Advertisement)
		require.True(t, ok)
		assert.Equal(t, DefaultHelmReleaseName, l2.Name)
		assert.Equal(t, DefaultHelmReleaseNamespace, l2.Namespace)
		assert.Equal(t, []string{DefaultHelmReleaseName}, l2.Spec.IPAddressPools)
	})
}

func TestApplyConfiguration(t *testing.T) {
	t.Parallel()

	validConfig := &v1alpha1.ServiceLoadBalancerConfiguration{
		AddressRanges: []v1alpha1.AddressRange{
			{Start: "10.100.1.1", End: "10.100.1.20"},
		},
	}

	t.Run("applies IPAddressPool and L2Advertisement with strict field validation", func(t *testing.T) {
		t.Parallel()

		var patchValidations []string
		remoteClient := fake.NewClientBuilder().
			WithScheme(testScheme(t)).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(
					ctx context.Context,
					cl ctrlclient.WithWatch,
					obj ctrlclient.Object,
					patch ctrlclient.Patch,
					opts ...ctrlclient.PatchOption,
				) error {
					patchOpts := &ctrlclient.PatchOptions{}
					patchOpts.ApplyOptions(opts)
					if patchOpts.Raw != nil {
						patchValidations = append(patchValidations, patchOpts.Raw.FieldValidation)
					}
					return cl.Patch(ctx, obj, patch, opts...)
				},
			}).
			Build()

		err := ApplyConfiguration(t.Context(), remoteClient, validConfig)
		require.NoError(t, err)
		assert.Equal(
			t,
			[]string{metav1.FieldValidationStrict, metav1.FieldValidationStrict},
			patchValidations,
		)

		var pool metallbv1.IPAddressPool
		require.NoError(
			t,
			remoteClient.Get(
				t.Context(),
				ctrlclient.ObjectKey{Namespace: DefaultHelmReleaseNamespace, Name: DefaultHelmReleaseName},
				&pool,
			),
		)
		assert.Equal(t, []string{"10.100.1.1-10.100.1.20"}, pool.Spec.Addresses)

		var l2 metallbv1.L2Advertisement
		require.NoError(
			t,
			remoteClient.Get(
				t.Context(),
				ctrlclient.ObjectKey{Namespace: DefaultHelmReleaseNamespace, Name: DefaultHelmReleaseName},
				&l2,
			),
		)
		assert.Equal(t, []string{DefaultHelmReleaseName}, l2.Spec.IPAddressPools)
	})

	t.Run("wraps IPAddressPool conflict with user-facing guidance", func(t *testing.T) {
		t.Parallel()

		remoteClient := fake.NewClientBuilder().
			WithScheme(testScheme(t)).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(
					_ context.Context,
					_ ctrlclient.WithWatch,
					obj ctrlclient.Object,
					_ ctrlclient.Patch,
					_ ...ctrlclient.PatchOption,
				) error {
					if _, ok := obj.(*metallbv1.IPAddressPool); ok {
						return apierrors.NewConflict(
							schema.GroupResource{Group: metallbv1.GroupVersion.Group, Resource: "ipaddresspools"},
							obj.GetName(),
							errors.New("Apply failed with 1 conflict"),
						)
					}
					return nil
				},
			}).
			Build()

		err := ApplyConfiguration(t.Context(), remoteClient, validConfig)
		require.Error(t, err)
		assert.True(t, apierrors.IsConflict(err))
		assert.ErrorContains(
			t,
			err,
			"failed to apply MetalLB configuration IPAddressPool metallb-system/metallb",
		)
		assert.ErrorContains(
			t,
			err,
			"This resource has been modified in the workload cluster: it must contain exactly the addresses listed in the Cluster configuration",
		)
	})

	t.Run("wraps L2Advertisement conflict with user-facing guidance", func(t *testing.T) {
		t.Parallel()

		remoteClient := fake.NewClientBuilder().
			WithScheme(testScheme(t)).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(
					ctx context.Context,
					cl ctrlclient.WithWatch,
					obj ctrlclient.Object,
					patch ctrlclient.Patch,
					opts ...ctrlclient.PatchOption,
				) error {
					if _, ok := obj.(*metallbv1.L2Advertisement); ok {
						return apierrors.NewConflict(
							schema.GroupResource{Group: metallbv1.GroupVersion.Group, Resource: "l2advertisements"},
							obj.GetName(),
							errors.New("Apply failed with 1 conflict"),
						)
					}
					return cl.Patch(ctx, obj, patch, opts...)
				},
			}).
			Build()

		err := ApplyConfiguration(t.Context(), remoteClient, validConfig)
		require.Error(t, err)
		assert.True(t, apierrors.IsConflict(err))
		assert.ErrorContains(
			t,
			err,
			"failed to apply MetalLB configuration L2Advertisement metallb-system/metallb",
		)
		assert.ErrorContains(
			t,
			err,
			`This resource has been modified in the workload cluster, it must only contain the "metallb" IP Address Pool`,
		)
	})

	t.Run("propagates internal webhook error for controller retry", func(t *testing.T) {
		t.Parallel()

		remoteClient := fake.NewClientBuilder().
			WithScheme(testScheme(t)).
			WithInterceptorFuncs(interceptor.Funcs{
				Patch: func(
					_ context.Context,
					_ ctrlclient.WithWatch,
					_ ctrlclient.Object,
					_ ctrlclient.Patch,
					_ ...ctrlclient.PatchOption,
				) error {
					return apierrors.NewInternalError(
						errors.New("failed calling webhook ipaddresspoolvalidationwebhook.metallb.io"),
					)
				},
			}).
			Build()

		err := ApplyConfiguration(t.Context(), remoteClient, validConfig)
		require.Error(t, err)
		assert.True(t, apierrors.IsInternalError(err))
	})
}

func TestMetalLBApplyDoesNotWaitOrApplyConfiguration(t *testing.T) {
	t.Parallel()

	scheme := testScheme(t)
	globalOpts := options.NewGlobalOptions()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	globalOpts.AddFlags(fs)
	cfg := &Config{GlobalOptions: globalOpts}
	cfg.AddFlags("metallb", fs)

	addonsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      globalOpts.HelmAddonsConfigMapName(),
			Namespace: globalOpts.DefaultsNamespace(),
		},
		Data: map[string]string{
			string(config.MetalLB): "ChartName: metallb\nChartVersion: 0.15.3\nRepositoryURL: https://metallb.github.io/metallb\n",
		},
	}
	valuesCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-metallb-helm-values-template",
			Namespace: globalOpts.DefaultsNamespace(),
		},
		Data: map[string]string{
			"values.yaml": "speaker:\n  frr:\n    enabled: false\n",
		},
	}
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workload-cluster",
			Namespace: corev1.NamespaceDefault,
			UID:       "cluster-uid-1",
			Annotations: map[string]string{
				v1alpha1.ClusterUUIDAnnotationKey: "uuid-1234",
			},
		},
	}

	mgmtClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(addonsCM, valuesCM, cluster).
		Build()
	remoteClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	handler := New(
		mgmtClient,
		cfg,
		config.NewHelmChartGetterFromConfigMap(
			globalOpts.HelmAddonsConfigMapName(),
			globalOpts.DefaultsNamespace(),
			mgmtClient,
		),
	)
	handler.remoteClientGetter = func(
		_ context.Context,
		_ ctrlclient.Client,
		_ ctrlclient.ObjectKey,
	) (ctrlclient.Client, error) {
		return remoteClient, nil
	}

	// Use a short deadline to prove Apply does not block waiting for HelmChartProxy Ready=True.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := handler.Apply(
		ctx,
		v1alpha1.ServiceLoadBalancer{
			Provider: v1alpha1.ServiceLoadBalancerProviderMetalLB,
			Configuration: &v1alpha1.ServiceLoadBalancerConfiguration{
				AddressRanges: []v1alpha1.AddressRange{
					{Start: "10.100.1.1", End: "10.100.1.20"},
				},
			},
		},
		cluster,
		logr.Discard(),
	)
	require.NoError(t, err)

	// Namespace must exist on the remote cluster with privileged PodSecurity labels.
	var ns corev1.Namespace
	require.NoError(
		t,
		remoteClient.Get(t.Context(), ctrlclient.ObjectKey{Name: DefaultHelmReleaseNamespace}, &ns),
	)
	assert.Equal(t, "privileged", ns.Labels["pod-security.kubernetes.io/enforce"])
	assert.Equal(t, "latest", ns.Labels["pod-security.kubernetes.io/enforce-version"])

	// HelmChartProxy must exist on the management cluster without needing Ready=True.
	var hcp caaphv1.HelmChartProxy
	require.NoError(
		t,
		mgmtClient.Get(
			t.Context(),
			ctrlclient.ObjectKey{
				Namespace: cluster.Namespace,
				Name:      HelmChartProxyName("uuid-1234"),
			},
			&hcp,
		),
	)
	assert.Equal(t, DefaultHelmReleaseName, hcp.Spec.ReleaseName)
	assert.Equal(t, DefaultHelmReleaseNamespace, hcp.Spec.ReleaseNamespace)

	// IPAddressPool and L2Advertisement must NOT be applied synchronously in the lifecycle hook;
	// they are reconciled asynchronously once the HelmChartProxy is Ready.
	var pools metallbv1.IPAddressPoolList
	require.NoError(t, remoteClient.List(t.Context(), &pools))
	assert.Empty(t, pools.Items)

	var l2s metallbv1.L2AdvertisementList
	require.NoError(t, remoteClient.List(t.Context(), &l2s))
	assert.Empty(t, l2s.Items)
}
