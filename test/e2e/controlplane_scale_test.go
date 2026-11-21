//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestClusterControlPlaneSettled(t *testing.T) {
	t.Parallel()

	settled := &clusterv1beta2.Cluster{
		Status: clusterv1beta2.ClusterStatus{
			Conditions: []metav1.Condition{
				{Type: clusterv1beta2.ClusterAvailableCondition, Status: metav1.ConditionTrue},
				{Type: clusterv1beta2.ClusterControlPlaneMachinesUpToDateCondition, Status: metav1.ConditionTrue},
				{Type: clusterv1beta2.ClusterRollingOutCondition, Status: metav1.ConditionFalse},
			},
		},
	}
	if !clusterControlPlaneSettled(settled) {
		t.Fatal("expected settled cluster")
	}

	rolling := settled.DeepCopy()
	for i := range rolling.Status.Conditions {
		if rolling.Status.Conditions[i].Type == clusterv1beta2.ClusterRollingOutCondition {
			rolling.Status.Conditions[i].Status = metav1.ConditionTrue
		}
	}
	if clusterControlPlaneSettled(rolling) {
		t.Fatal("rolling cluster is not settled")
	}

	readyOnly := &clusterv1beta2.Cluster{
		Status: clusterv1beta2.ClusterStatus{
			Conditions: []metav1.Condition{
				{Type: clusterv1beta2.ReadyCondition, Status: metav1.ConditionTrue},
			},
		},
	}
	if clusterControlPlaneSettled(readyOnly) {
		t.Fatal("v1beta2 Cluster Ready is not the settled signal")
	}
}

func TestKubeadmControlPlaneAtReplicas(t *testing.T) {
	t.Parallel()

	one := ptr.To(int32(1))
	two := ptr.To(int32(2))
	tests := []struct {
		name string
		kcp  controlplanev1.KubeadmControlPlane
		want bool
	}{
		{
			name: "spec status and ready all one",
			kcp: controlplanev1.KubeadmControlPlane{
				Spec: controlplanev1.KubeadmControlPlaneSpec{Replicas: one},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:      one,
					ReadyReplicas: one,
				},
			},
			want: true,
		},
		{
			name: "nil spec",
			kcp: controlplanev1.KubeadmControlPlane{
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:      one,
					ReadyReplicas: one,
				},
			},
		},
		{
			name: "nil status replicas",
			kcp: controlplanev1.KubeadmControlPlane{
				Spec:   controlplanev1.KubeadmControlPlaneSpec{Replicas: one},
				Status: controlplanev1.KubeadmControlPlaneStatus{ReadyReplicas: one},
			},
		},
		{
			name: "nil ready replicas",
			kcp: controlplanev1.KubeadmControlPlane{
				Spec:   controlplanev1.KubeadmControlPlaneSpec{Replicas: one},
				Status: controlplanev1.KubeadmControlPlaneStatus{Replicas: one},
			},
		},
		{
			name: "spec still two while status is one",
			kcp: controlplanev1.KubeadmControlPlane{
				Spec: controlplanev1.KubeadmControlPlaneSpec{Replicas: two},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:      one,
					ReadyReplicas: one,
				},
			},
		},
		{
			name: "ready lags observed",
			kcp: controlplanev1.KubeadmControlPlane{
				Spec: controlplanev1.KubeadmControlPlaneSpec{Replicas: one},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:      one,
					ReadyReplicas: two,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := kubeadmControlPlaneAtReplicas(&tt.kcp, 1); got != tt.want {
				t.Fatalf("kubeadmControlPlaneAtReplicas() = %v, want %v", got, tt.want)
			}
		})
	}
}
