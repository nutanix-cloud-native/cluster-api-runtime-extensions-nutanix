//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
)

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
