// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconciler_calculateFailureDomainHash(t *testing.T) {
	r := &Reconciler{}

	tests := []struct {
		name           string
		failureDomains clusterv1.FailureDomains
		expectedHash   string
		expectedUnique bool
	}{
		{
			name:           "empty failure domains",
			failureDomains: clusterv1.FailureDomains{},
			expectedHash:   "",
		},
		{
			name: "single control plane failure domain",
			failureDomains: clusterv1.FailureDomains{
				"fd1": clusterv1.FailureDomainSpec{
					ControlPlane: true,
				},
			},
			expectedHash: "fd1,",
		},
		{
			name: "multiple control plane failure domains",
			failureDomains: clusterv1.FailureDomains{
				"fd2": clusterv1.FailureDomainSpec{
					ControlPlane: true,
				},
				"fd1": clusterv1.FailureDomainSpec{
					ControlPlane: true,
				},
			},
			expectedHash: "fd1,fd2,", // Should be sorted
		},
		{
			name: "mixed failure domains",
			failureDomains: clusterv1.FailureDomains{
				"fd1": clusterv1.FailureDomainSpec{
					ControlPlane: true,
				},
				"fd2": clusterv1.FailureDomainSpec{
					ControlPlane: false, // Not control plane
				},
				"fd3": clusterv1.FailureDomainSpec{
					ControlPlane: true,
				},
			},
			expectedHash: "fd1,fd3,", // Only control plane domains
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := r.calculateFailureDomainHash(tt.failureDomains)
			require.Equal(t, tt.expectedHash, hash)
		})
	}
}

func TestReconciler_shouldTriggerRollout(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name            string
		cluster         *clusterv1.Cluster
		kcp             *controlplanev1.KubeadmControlPlane
		machines        []clusterv1.Machine
		expectedRollout bool
		expectedReason  string
	}{
		{
			name: "no previous hash - should not trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{
							ControlPlane: true,
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expectedRollout: false,
		},
		{
			name: "same failure domains - should not trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{
							ControlPlane: true,
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						FailureDomainHashAnnotation: "fd1,",
					},
				},
			},
			expectedRollout: false,
		},
		{
			name: "used failure domain removed - should trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd2": clusterv1.FailureDomainSpec{
							ControlPlane: true,
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						FailureDomainHashAnnotation: "fd1,fd2,",
					},
				},
			},
			machines: []clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         "test-cluster",
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: clusterv1.MachineSpec{
						FailureDomain: ptr.To("fd1"), // Using fd1 which is now removed
					},
				},
			},
			expectedRollout: true,
			expectedReason:  "failure domain fd1 is removed",
		},
		{
			name: "used failure domain disabled - should trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{
							ControlPlane: false, // Disabled for control plane
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						FailureDomainHashAnnotation: "fd1,",
					},
				},
			},
			machines: []clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         "test-cluster",
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: clusterv1.MachineSpec{
						FailureDomain: ptr.To("fd1"), // Using fd1 which is now disabled
					},
				},
			},
			expectedRollout: true,
			expectedReason:  "failure domain fd1 is disabled for control plane",
		},
		{
			name: "new failure domain added - should not trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{
							ControlPlane: true,
						},
						"fd2": clusterv1.FailureDomainSpec{
							ControlPlane: true,
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						FailureDomainHashAnnotation: "fd1,", // Only fd1 was present before
					},
				},
			},
			machines: []clusterv1.Machine{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-1",
						Namespace: "test-namespace",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         "test-cluster",
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: clusterv1.MachineSpec{
						FailureDomain: ptr.To("fd1"), // Using fd1 which is still available
					},
				},
			},
			expectedRollout: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []client.Object{tt.cluster, tt.kcp}
			for i := range tt.machines {
				objs = append(objs, &tt.machines[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			r := &Reconciler{
				Client: fakeClient,
			}

			needsRollout, reason, err := r.shouldTriggerRollout(context.Background(), tt.cluster, tt.kcp)
			require.NoError(t, err)
			require.Equal(t, tt.expectedRollout, needsRollout)
			if tt.expectedReason != "" {
				require.Equal(t, tt.expectedReason, reason)
			}
		})
	}
}

func TestReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{}, // Has topology
			ControlPlaneRef: &corev1.ObjectReference{
				Name: "test-kcp",
			},
		},
		Status: clusterv1.ClusterStatus{
			FailureDomains: clusterv1.FailureDomains{
				"fd1": clusterv1.FailureDomainSpec{
					ControlPlane: false, // Disabled
				},
			},
		},
	}

	kcp := &controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-kcp",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				FailureDomainHashAnnotation: "fd1,", // Had fd1 enabled before
			},
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: ptr.To[int32](3),
		},
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-machine-1",
			Namespace: "test-namespace",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:         "test-cluster",
				clusterv1.MachineControlPlaneLabel: "",
			},
		},
		Spec: clusterv1.MachineSpec{
			FailureDomain: ptr.To("fd1"),
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, kcp, machine).
		Build()

	r := &Reconciler{
		Client: fakeClient,
	}

	req := reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(cluster),
	}

	_, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)

	// Verify that the KCP was updated with rolloutAfter
	var updatedKCP controlplanev1.KubeadmControlPlane
	err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(kcp), &updatedKCP)
	require.NoError(t, err)

	require.NotNil(t, updatedKCP.Spec.RolloutAfter)
	require.WithinDuration(t, time.Now(), updatedKCP.Spec.RolloutAfter.Time, 5*time.Second)
	require.Contains(t, updatedKCP.Annotations, FailureDomainLastUpdateAnnotation)
}
