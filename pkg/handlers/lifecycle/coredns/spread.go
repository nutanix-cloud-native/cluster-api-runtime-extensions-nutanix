// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package coredns

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DeploymentName is the kubeadm-managed CoreDNS Deployment name.
	DeploymentName = "coredns"
	// DeploymentNamespace is where kubeadm installs CoreDNS.
	DeploymentNamespace = "kube-system"
	// AppLabelKey / AppLabelValue select CoreDNS pods (kubeadm default).
	AppLabelKey   = "k8s-app"
	AppLabelValue = "kube-dns"

	topologyKeyHostname = "kubernetes.io/hostname"
	// topologyKeyZone is the fault domain for both non-metro and metro clusters.
	// Non-metro nodes are labeled with the Prism Element or category zone.
	// Metro control-plane nodes use a NutanixMetroSite failure domain, and the
	// cloud provider copies that failure-domain name into this label, so two
	// sites become two zones. A NutanixMetro failure domain is one zone shared
	// by both sites and does not split pods.
	topologyKeyZone = "topology.kubernetes.io/zone"

	// spreadRolloutAnnotation bumps the pod template to trigger a Deployment
	// rollout after required zone anti-affinity is applied. Pods are never deleted
	// directly by the handler.
	spreadRolloutAnnotation = "coredns.caren.nutanix.com/fault-domain-spread"
	// spreadRolloutAnnotationValue is set when a rollout is requested for the
	// required zone anti-affinity policy.
	spreadRolloutAnnotationValue = "zone-required"
)

const controlPlaneNodeLabel = "node-role.kubernetes.io/control-plane"

func coreDNSPodAffinityTerm() corev1.PodAffinityTerm {
	return corev1.PodAffinityTerm{
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      AppLabelKey,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{AppLabelValue},
				},
			},
		},
		TopologyKey: "", // set by caller
	}
}

// isMultiZoneCluster reports whether Ready nodes expose at least two distinct zone values.
func isMultiZoneCluster(nodes []corev1.Node) bool {
	return hasMultipleDistinctValues(nodes, topologyKeyZone)
}

// buildMultiZonePodAntiAffinity returns required zone + preferred hostname anti-affinity.
// Call only for multi-zone clusters. Returns nil for single-zone (caller should no-op).
func buildMultiZonePodAntiAffinity(nodes []corev1.Node) *corev1.PodAntiAffinity {
	if !isMultiZoneCluster(nodes) {
		return nil
	}

	zoneTerm := coreDNSPodAffinityTerm()
	zoneTerm.TopologyKey = topologyKeyZone

	hostTerm := coreDNSPodAffinityTerm()
	hostTerm.TopologyKey = topologyKeyHostname

	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{zoneTerm},
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
			{
				Weight:          1,
				PodAffinityTerm: hostTerm,
			},
		},
	}
}

func hasMultipleDistinctValues(nodes []corev1.Node, labelKey string) bool {
	values := map[string]struct{}{}
	for i := range nodes {
		node := &nodes[i]
		if !isNodeReady(node) {
			continue
		}
		v, ok := node.Labels[labelKey]
		if !ok || v == "" {
			continue
		}
		values[v] = struct{}{}
		if len(values) >= 2 {
			return true
		}
	}
	return false
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func countReadyControlPlaneNodes(nodes []corev1.Node) int32 {
	var count int32
	for i := range nodes {
		node := &nodes[i]
		if !isNodeReady(node) {
			continue
		}
		if _, ok := node.Labels[controlPlaneNodeLabel]; !ok {
			continue
		}
		count++
	}
	return count
}

// affinityNeedsUpdate reports whether the Deployment affinity should be patched
// to match the desired multi-zone anti-affinity.
func affinityNeedsUpdate(current *corev1.Affinity, desired *corev1.PodAntiAffinity) bool {
	if desired == nil {
		return false
	}
	if current == nil || current.PodAntiAffinity == nil {
		return true
	}
	cur := current.PodAntiAffinity
	want := desired

	if len(
		cur.RequiredDuringSchedulingIgnoredDuringExecution,
	) != len(
		want.RequiredDuringSchedulingIgnoredDuringExecution,
	) {
		return true
	}
	for i := range want.RequiredDuringSchedulingIgnoredDuringExecution {
		if cur.RequiredDuringSchedulingIgnoredDuringExecution[i].TopologyKey !=
			want.RequiredDuringSchedulingIgnoredDuringExecution[i].TopologyKey {
			return true
		}
	}

	if len(
		cur.PreferredDuringSchedulingIgnoredDuringExecution,
	) != len(
		want.PreferredDuringSchedulingIgnoredDuringExecution,
	) {
		return true
	}
	for i := range want.PreferredDuringSchedulingIgnoredDuringExecution {
		if cur.PreferredDuringSchedulingIgnoredDuringExecution[i].Weight !=
			want.PreferredDuringSchedulingIgnoredDuringExecution[i].Weight {
			return true
		}
		if cur.PreferredDuringSchedulingIgnoredDuringExecution[i].PodAffinityTerm.TopologyKey !=
			want.PreferredDuringSchedulingIgnoredDuringExecution[i].PodAffinityTerm.TopologyKey {
			return true
		}
	}
	return false
}

// podsSpreadAcrossZones reports whether running CoreDNS pods use at least two distinct zones.
// Pods without a nodeName or whose node has no zone label are ignored for the count.
func podsSpreadAcrossZones(pods []corev1.Pod, nodes []corev1.Node) bool {
	nodeZone := map[string]string{}
	for i := range nodes {
		node := &nodes[i]
		if z, ok := node.Labels[topologyKeyZone]; ok && z != "" {
			nodeZone[node.Name] = z
		}
	}

	zones := map[string]struct{}{}
	for i := range pods {
		pod := &pods[i]
		if pod.DeletionTimestamp != nil {
			continue
		}
		if pod.Spec.NodeName == "" {
			continue
		}
		z, ok := nodeZone[pod.Spec.NodeName]
		if !ok {
			continue
		}
		zones[z] = struct{}{}
		if len(zones) >= 2 {
			return true
		}
	}
	return false
}

func needsSpreadRollout(deployAnnotations map[string]string, pods []corev1.Pod, nodes []corev1.Node) bool {
	if podsSpreadAcrossZones(pods, nodes) {
		return false
	}
	if deployAnnotations != nil && deployAnnotations[spreadRolloutAnnotation] == spreadRolloutAnnotationValue {
		// Rollout already requested for this policy; do not keep bumping.
		return false
	}
	return true
}
