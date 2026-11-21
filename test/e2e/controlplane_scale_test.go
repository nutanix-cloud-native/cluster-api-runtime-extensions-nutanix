//go:build e2e

// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/generic/mutation/kubeadm/authentication"
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

func TestAuthenticationConfigCurrent(t *testing.T) {
	t.Parallel()

	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: ptr.To(int32(1)),
			KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
				Files: []bootstrapv1.File{{
					Path: authentication.AuthenticationConfigPath,
				}},
			},
		},
		Status: controlplanev1.KubeadmControlPlaneStatus{
			ObservedGeneration: 3,
			Replicas:           ptr.To(int32(1)),
			ReadyReplicas:      ptr.To(int32(1)),
		},
	}
	if !authenticationConfigCurrent(kcp) {
		t.Fatal("expected current authentication config")
	}

	beforeReconcile := kcp.DeepCopy()
	beforeReconcile.Status.ObservedGeneration = 2
	if authenticationConfigCurrent(beforeReconcile) {
		t.Fatal("unobserved generation is not current")
	}

	noFile := kcp.DeepCopy()
	noFile.Spec.KubeadmConfigSpec.Files = nil
	if authenticationConfigCurrent(noFile) {
		t.Fatal("missing authentication file is not current")
	}
}

func TestBearerOnlyConfigDropsAdminCert(t *testing.T) {
	t.Parallel()

	cfg := &rest.Config{
		Host:     "https://apiserver",
		Username: "kubernetes-admin",
		Password: "secret",
	}
	cfg.BearerToken = "admin-token"
	cfg.BearerTokenFile = "/tmp/token"
	cfg.CertFile = "/tmp/admin.crt"
	cfg.KeyFile = "/tmp/admin.key"
	cfg.CertData = []byte("cert")
	cfg.KeyData = []byte("key")
	out := bearerOnlyConfig(cfg, "jwt")
	if out.BearerToken != "jwt" || out.CertData != nil || out.KeyData != nil ||
		out.CertFile != "" || out.KeyFile != "" || out.Username != "" || out.Password != "" ||
		out.BearerTokenFile != "" {
		t.Fatalf("admin credentials leaked into bearer config: %#v", out)
	}
	if cfg.CertData == nil || cfg.Username != "kubernetes-admin" {
		t.Fatal("bearerOnlyConfig mutated the input config")
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
