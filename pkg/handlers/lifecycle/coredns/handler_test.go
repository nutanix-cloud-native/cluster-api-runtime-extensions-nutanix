// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureCoreDNSFaultDomainSpread_patchesMultiZoneRequired(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: topologyKeyHostname,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		deploy,
		readyNode("n1", map[string]string{topologyKeyZone: "site-a"}),
		readyNode("n2", map[string]string{topologyKeyZone: "site-b"}),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns-1",
				Namespace: DeploymentNamespace,
				Labels:    map[string]string{AppLabelKey: AppLabelValue},
			},
			Spec: corev1.PodSpec{NodeName: "n1"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns-2",
				Namespace: DeploymentNamespace,
				Labels:    map[string]string{AppLabelKey: AppLabelValue},
			},
			Spec: corev1.PodSpec{NodeName: "n1"},
		},
	).Build()

	require.NoError(t, ensureCoreDNSFaultDomainSpread(context.Background(), c, 1, false))

	updated := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: DeploymentName, Namespace: DeploymentNamespace,
	}, updated))

	paa := updated.Spec.Template.Spec.Affinity.PodAntiAffinity
	require.Len(t, paa.RequiredDuringSchedulingIgnoredDuringExecution, 1)
	assert.Equal(t, topologyKeyZone, paa.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey)
	require.Len(t, paa.PreferredDuringSchedulingIgnoredDuringExecution, 1)
	assert.Equal(t, topologyKeyHostname, paa.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey)
	assert.Equal(t, spreadRolloutAnnotationValue, updated.Spec.Template.Annotations[spreadRolloutAnnotation])
}

func TestEnsureCoreDNSFaultDomainSpread_singleZoneNoop(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeploymentName,
			Namespace: DeploymentNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"keep": "me"},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: topologyKeyHostname,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		deploy,
		readyNode("n1", map[string]string{topologyKeyZone: "pe-a"}),
		readyNode("n2", map[string]string{topologyKeyZone: "pe-a"}),
	).Build()

	require.NoError(t, ensureCoreDNSFaultDomainSpread(context.Background(), c, 1, false))

	updated := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: DeploymentName, Namespace: DeploymentNamespace,
	}, updated))

	assert.Equal(t, "me", updated.Spec.Template.Annotations["keep"])
	terms := updated.Spec.Template.Spec.Affinity.PodAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	require.Len(t, terms, 1)
	assert.Equal(t, topologyKeyHostname, terms[0].PodAffinityTerm.TopologyKey)
	assert.Empty(t, updated.Spec.Template.Spec.Affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
	assert.NotContains(t, updated.Spec.Template.Annotations, spreadRolloutAnnotation)
}

func TestEnsureCoreDNSFaultDomainSpread_notFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		readyNode("n1", map[string]string{topologyKeyZone: "a"}),
		readyNode("n2", map[string]string{topologyKeyZone: "b"}),
	).Build()
	err := ensureCoreDNSFaultDomainSpread(context.Background(), c, 1, false)
	require.Error(t, err)
}

func TestEnsureCoreDNSFaultDomainSpread_waitsForControlPlane(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: DeploymentNamespace},
		},
		readyControlPlaneNode("cp1", map[string]string{topologyKeyZone: "NutanixMetroSite_metro0-s0"}),
	).Build()

	err := ensureCoreDNSFaultDomainSpread(context.Background(), c, 3, true)
	require.ErrorIs(t, err, errControlPlaneNodesPending)
}

func TestEnsureCoreDNSFaultDomainSpread_succeedsWhenAllControlPlanesReady(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: DeploymentNamespace},
		},
		readyControlPlaneNode("cp1", map[string]string{topologyKeyZone: "NutanixMetroSite_metro0-s0"}),
		readyControlPlaneNode("cp2", map[string]string{topologyKeyZone: "NutanixMetroSite_metro0-s0"}),
		readyControlPlaneNode("cp3", map[string]string{topologyKeyZone: "NutanixMetroSite_metro0-s1"}),
		readyNode("w1", map[string]string{topologyKeyZone: "NutanixMetro_metro0"}),
	).Build()

	require.NoError(t, ensureCoreDNSFaultDomainSpread(context.Background(), c, 3, true))

	updated := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: DeploymentName, Namespace: DeploymentNamespace,
	}, updated))
	paa := updated.Spec.Template.Spec.Affinity.PodAntiAffinity
	require.Len(t, paa.RequiredDuringSchedulingIgnoredDuringExecution, 1)
	assert.Equal(t, topologyKeyZone, paa.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey)
}

func TestEnsureCoreDNSFaultDomainSpread_skipsRolloutWhenAlreadySpread(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	desired := buildMultiZonePodAntiAffinity([]corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "b"}),
	})

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: DeploymentNamespace},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{PodAntiAffinity: desired},
				},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		deploy,
		readyNode("n1", map[string]string{topologyKeyZone: "a"}),
		readyNode("n2", map[string]string{topologyKeyZone: "b"}),
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "coredns-1", Namespace: DeploymentNamespace,
				Labels: map[string]string{AppLabelKey: AppLabelValue},
			},
			Spec: corev1.PodSpec{NodeName: "n1"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "coredns-2", Namespace: DeploymentNamespace,
				Labels: map[string]string{AppLabelKey: AppLabelValue},
			},
			Spec: corev1.PodSpec{NodeName: "n2"},
		},
	).Build()

	require.NoError(t, ensureCoreDNSFaultDomainSpread(context.Background(), c, 1, false))

	updated := &appsv1.Deployment{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: DeploymentName, Namespace: DeploymentNamespace,
	}, updated))
	assert.NotContains(t, updated.Spec.Template.Annotations, spreadRolloutAnnotation)
}

func TestHandlerAfterControlPlaneInitialized_retriesUntilControlPlanesReady(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	remote := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: DeploymentNamespace},
		},
		readyControlPlaneNode("cp1", map[string]string{topologyKeyZone: "NutanixMetroSite_metro0-s0"}),
	).Build()

	mgmt := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := &Handler{
		client: mgmt,
		clusterClientGetter: func(
			_ context.Context,
			_ string,
			_ client.Client,
			_ client.ObjectKey,
		) (client.Client, error) {
			return remote, nil
		},
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
		Spec: clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{Name: "nutanix-class"},
				Version:  "v1.37.1",
				ControlPlane: clusterv1.ControlPlaneTopology{
					Replicas: ptr.To[int32](3),
				},
			},
		},
	}
	req := &runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster}
	resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

	h.AfterControlPlaneInitialized(context.Background(), req, resp)
	assert.Equal(t, runtimehooksv1.ResponseStatusFailure, resp.GetStatus())
	assert.Contains(t, resp.GetMessage(), "waiting for remaining control-plane nodes")
}

func TestHandlerAfterControlPlaneInitialized_success(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, clusterv1.AddToScheme(scheme))

	remote := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: DeploymentName, Namespace: DeploymentNamespace},
		},
		readyNode("n1", map[string]string{topologyKeyZone: "a"}),
		readyNode("n2", map[string]string{topologyKeyZone: "b"}),
	).Build()

	mgmt := fake.NewClientBuilder().WithScheme(scheme).Build()
	h := &Handler{
		client: mgmt,
		clusterClientGetter: func(
			_ context.Context,
			_ string,
			_ client.Client,
			_ client.ObjectKey,
		) (client.Client, error) {
			return remote, nil
		},
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c1", Namespace: "default"},
	}
	req := &runtimehooksv1.AfterControlPlaneInitializedRequest{Cluster: *cluster}
	resp := &runtimehooksv1.AfterControlPlaneInitializedResponse{}

	h.AfterControlPlaneInitialized(context.Background(), req, resp)
	assert.Equal(t, runtimehooksv1.ResponseStatusSuccess, resp.GetStatus())
}

func readyControlPlaneNode(name string, labels map[string]string) *corev1.Node {
	if labels == nil {
		labels = map[string]string{}
	}
	labels[controlPlaneNodeLabel] = ""
	return readyNode(name, labels)
}
