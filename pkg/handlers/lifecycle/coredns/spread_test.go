// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func readyNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
}

func TestIsMultiZoneCluster_detectsMultipleFailureDomains(t *testing.T) {
	assert.False(t, isMultiZoneCluster([]corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-a"}),
	}), "one shared failure domain must be treated as single-domain")
	assert.True(t, isMultiZoneCluster([]corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-b"}),
	}), "distinct topology.kubernetes.io/zone values are distinct failure domains")
}

func TestBuildMultiZonePodAntiAffinity_nilWhenSingleFailureDomain(t *testing.T) {
	nodes := []corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-a"}),
	}
	assert.Nil(t, buildMultiZonePodAntiAffinity(nodes))
}

func TestBuildMultiZonePodAntiAffinity_requiredZoneWhenMultipleFailureDomains(t *testing.T) {
	nodes := []corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-b"}),
	}

	anti := buildMultiZonePodAntiAffinity(nodes)
	require.NotNil(t, anti)
	require.Len(t, anti.RequiredDuringSchedulingIgnoredDuringExecution, 1)
	assert.Equal(t, topologyKeyZone, anti.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey)
	assert.Equal(
		t,
		AppLabelKey,
		anti.RequiredDuringSchedulingIgnoredDuringExecution[0].LabelSelector.MatchExpressions[0].Key,
	)

	require.Len(t, anti.PreferredDuringSchedulingIgnoredDuringExecution, 1)
	assert.Equal(
		t,
		topologyKeyHostname,
		anti.PreferredDuringSchedulingIgnoredDuringExecution[0].PodAffinityTerm.TopologyKey,
	)
	assert.Equal(t, int32(1), anti.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight)
}

func TestBuildMultiZonePodAntiAffinity_usesFailureDomainZoneLabels(t *testing.T) {
	// Zone label values are the CAPI failure-domain names providers copy onto Nodes.
	nodes := []corev1.Node{
		*readyNode("cp-1", map[string]string{topologyKeyZone: "fd-east"}),
		*readyNode("cp-2", map[string]string{topologyKeyZone: "fd-west"}),
		*readyNode("w-1", map[string]string{topologyKeyZone: "fd-shared"}),
	}

	anti := buildMultiZonePodAntiAffinity(nodes)
	require.NotNil(t, anti)
	require.Len(t, anti.RequiredDuringSchedulingIgnoredDuringExecution, 1)
	assert.Equal(t, topologyKeyZone, anti.RequiredDuringSchedulingIgnoredDuringExecution[0].TopologyKey)
}

func TestBuildMultiZonePodAntiAffinity_sharedFailureDomainIsNil(t *testing.T) {
	// All Ready nodes share one failure-domain zone label -> no required anti-affinity.
	nodes := []corev1.Node{
		*readyNode("w-1", map[string]string{topologyKeyZone: "fd-shared"}),
		*readyNode("w-2", map[string]string{topologyKeyZone: "fd-shared"}),
	}
	assert.Nil(t, buildMultiZonePodAntiAffinity(nodes))
}

func TestBuildMultiZonePodAntiAffinity_ignoresNotReadyNodes(t *testing.T) {
	notReady := readyNode("n2", map[string]string{topologyKeyZone: "fd-b"})
	notReady.Status.Conditions[0].Status = corev1.ConditionFalse

	nodes := []corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*notReady,
	}
	assert.Nil(t, buildMultiZonePodAntiAffinity(nodes))
}

func TestAffinityNeedsUpdate(t *testing.T) {
	desired := buildMultiZonePodAntiAffinity([]corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-b"}),
	})

	assert.True(t, affinityNeedsUpdate(nil, desired))
	assert.True(t, affinityNeedsUpdate(&corev1.Affinity{}, desired))
	assert.False(t, affinityNeedsUpdate(&corev1.Affinity{PodAntiAffinity: desired}, desired))

	kubeadmDefault := &corev1.Affinity{
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
	}
	assert.True(t, affinityNeedsUpdate(kubeadmDefault, desired))
}

func TestPodsSpreadAcrossZones(t *testing.T) {
	nodes := []corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-b"}),
	}
	sameFailureDomain := []corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: "n1"}},
		{Spec: corev1.PodSpec{NodeName: "n1"}},
	}
	assert.False(t, podsSpreadAcrossZones(sameFailureDomain, nodes))

	spreadAcrossFailureDomains := []corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: "n1"}},
		{Spec: corev1.PodSpec{NodeName: "n2"}},
	}
	assert.True(t, podsSpreadAcrossZones(spreadAcrossFailureDomains, nodes))
}

func TestNeedsSpreadRollout(t *testing.T) {
	nodes := []corev1.Node{
		*readyNode("n1", map[string]string{topologyKeyZone: "fd-a"}),
		*readyNode("n2", map[string]string{topologyKeyZone: "fd-b"}),
	}
	unspread := []corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: "n1"}},
		{Spec: corev1.PodSpec{NodeName: "n1"}},
	}

	assert.True(t, needsSpreadRollout(nil, unspread, nodes))
	assert.False(t, needsSpreadRollout(map[string]string{
		spreadRolloutAnnotation: spreadRolloutAnnotationValue,
	}, unspread, nodes))

	spread := []corev1.Pod{
		{Spec: corev1.PodSpec{NodeName: "n1"}},
		{Spec: corev1.PodSpec{NodeName: "n2"}},
	}
	assert.False(t, needsSpreadRollout(nil, spread, nodes))
}
