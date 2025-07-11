// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

func TestReconciler_shouldImproveDistribution(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name                    string
		replicas                int32
		machines                []clusterv1.Machine
		availableFailureDomains []string
		expectedImprove         bool
		description             string
	}{
		// 1 replica tests
		{
			name:     "1 replica, 1 FD - already optimal",
			replicas: 1,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
			},
			availableFailureDomains: []string{"fd1"},
			expectedImprove:         false,
			description:             "1 replica on 1 FD is optimal",
		},
		{
			name:     "1 replica, 1 FD, new FD added - no improvement possible",
			replicas: 1,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
			},
			availableFailureDomains: []string{"fd1", "fd2"},
			expectedImprove:         false,
			description:             "adding FDs doesn't help with 1 replica",
		},

		// 3 replica tests
		{
			name:     "3 replicas, 1 FD - should improve when 2nd FD added",
			replicas: 3,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd1"),
			},
			availableFailureDomains: []string{"fd1", "fd2"},
			expectedImprove:         true,
			description:             "3 replicas on 1 FD can improve to [2,1]",
		},
		{
			name:     "3 replicas, 2 FDs [2,1] - should improve when 3rd FD added",
			replicas: 3,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd2"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3"},
			expectedImprove:         true,
			description:             "3 replicas [2,1] can improve to [1,1,1]",
		},
		{
			name:     "3 replicas, 3 FDs [1,1,1] - already optimal",
			replicas: 3,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
				createMachine("m3", "fd3"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3"},
			expectedImprove:         false,
			description:             "3 replicas evenly distributed is optimal",
		},
		{
			name:     "3 replicas, 3 FDs [1,1,1], 4th FD added - no improvement needed",
			replicas: 3,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
				createMachine("m3", "fd3"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3", "fd4"},
			expectedImprove:         false,
			description:             "3 replicas [1,1,1] is still optimal with 4th FD",
		},

		// 5 replica tests
		{
			name:     "5 replicas, 1 FD - should improve when 2nd FD added",
			replicas: 5,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd1"),
				createMachine("m4", "fd1"),
				createMachine("m5", "fd1"),
			},
			availableFailureDomains: []string{"fd1", "fd2"},
			expectedImprove:         true,
			description:             "5 replicas on 1 FD can improve to [3,2]",
		},
		{
			name:     "5 replicas, 2 FDs [3,2] - should improve when 3rd FD added",
			replicas: 5,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd1"),
				createMachine("m4", "fd2"),
				createMachine("m5", "fd2"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3"},
			expectedImprove:         true,
			description:             "5 replicas [3,2] can improve to [2,2,1]",
		},
		{
			name:     "5 replicas, 3 FDs [2,2,1] - should improve when 4th FD added",
			replicas: 5,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd2"),
				createMachine("m4", "fd2"),
				createMachine("m5", "fd3"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3", "fd4"},
			expectedImprove:         true,
			description:             "5 replicas [2,2,1] can improve to [2,1,1,1]",
		},
		{
			name:     "5 replicas, 4 FDs [2,1,1,1] - should improve when 5th FD added",
			replicas: 5,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd2"),
				createMachine("m4", "fd3"),
				createMachine("m5", "fd4"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3", "fd4", "fd5"},
			expectedImprove:         true,
			description:             "5 replicas [2,1,1,1] can improve to [1,1,1,1,1]",
		},
		{
			name:     "5 replicas, 5 FDs [1,1,1,1,1] - already optimal",
			replicas: 5,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
				createMachine("m3", "fd3"),
				createMachine("m4", "fd4"),
				createMachine("m5", "fd5"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3", "fd4", "fd5"},
			expectedImprove:         false,
			description:             "5 replicas evenly distributed is optimal",
		},
		{
			name:     "5 replicas, 5 FDs [1,1,1,1,1], 6th FD added - no improvement needed",
			replicas: 5,
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
				createMachine("m3", "fd3"),
				createMachine("m4", "fd4"),
				createMachine("m5", "fd5"),
			},
			availableFailureDomains: []string{"fd1", "fd2", "fd3", "fd4", "fd5", "fd6"},
			expectedImprove:         false,
			description:             "5 replicas [1,1,1,1,1] is still optimal with 6th FD",
		},

		// Edge cases
		{
			name:                    "no replicas specified",
			replicas:                0,
			machines:                []clusterv1.Machine{},
			availableFailureDomains: []string{"fd1", "fd2"},
			expectedImprove:         false,
			description:             "0 replicas should not trigger improvement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			}

			kcp := &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: &tt.replicas,
				},
			}

			objs := []client.Object{cluster, kcp}
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

			// Get current distribution from the fake client
			currentDistribution, err := r.getMachineDistribution(context.Background(), cluster)
			require.NoError(t, err)

			shouldImprove := r.shouldImproveDistribution(
				kcp,
				currentDistribution,
				tt.availableFailureDomains,
			)
			require.Equal(t, tt.expectedImprove, shouldImprove, "Test case: %s", tt.description)
		})
	}
}

func TestReconciler_shouldSkipRollout(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name                 string
		kcp                  *controlplanev1.KubeadmControlPlane
		expectedSkip         bool
		expectedRequeueAfter time.Duration
		description          string
	}{
		{
			name: "no rollout after set - should not skip",
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					// No RolloutAfter set
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:        3,
					UpdatedReplicas: 3,
				},
			},
			expectedSkip:         false,
			expectedRequeueAfter: 0,
			description:          "should not skip when no rollout is in progress",
		},
		{
			name: "recent rollout after set - should skip",
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					RolloutAfter: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:        3,
					UpdatedReplicas: 3,
				},
			},
			expectedSkip:         true,
			expectedRequeueAfter: 5 * time.Minute,
			description:          "should skip when rollout was triggered recently",
		},
		{
			name: "old rollout after set - should not skip",
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					RolloutAfter: &metav1.Time{Time: time.Now().Add(-25 * time.Minute)},
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:        3,
					UpdatedReplicas: 3,
				},
			},
			expectedSkip:         false,
			expectedRequeueAfter: 0,
			description:          "should not skip when rollout was triggered long ago",
		},
		{
			name: "rollout in progress - should skip",
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					// No RolloutAfter set
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:        3,
					UpdatedReplicas: 1, // Rollout in progress
				},
			},
			expectedSkip:         true,
			expectedRequeueAfter: 2 * time.Minute,
			description:          "should skip when rollout is in progress (updated < replicas)",
		},
		{
			name: "machines not up to date - should skip",
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					// No RolloutAfter set
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:        3,
					UpdatedReplicas: 3,
					Conditions: []clusterv1.Condition{
						{
							Type:   controlplanev1.MachinesSpecUpToDateCondition,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expectedSkip:         true,
			expectedRequeueAfter: 2 * time.Minute,
			description:          "should skip when machines are not up to date",
		},
		{
			name: "multiple skip conditions - should skip with shortest requeue",
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					RolloutAfter: &metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					Replicas:        3,
					UpdatedReplicas: 1, // Also in progress
				},
			},
			expectedSkip:         true,
			expectedRequeueAfter: 5 * time.Minute, // First check returns first
			description:          "should skip with first applicable condition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{}

			shouldSkip, requeueAfter := r.shouldSkipRollout(tt.kcp)

			require.Equal(t, tt.expectedSkip, shouldSkip, "Test case: %s", tt.description)
			require.Equal(t, tt.expectedRequeueAfter, requeueAfter, "Test case: %s", tt.description)
		})
	}
}

func TestReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name                   string
		cluster                *clusterv1.Cluster
		kcp                    *controlplanev1.KubeadmControlPlane
		machines               []clusterv1.Machine
		expectRolloutTriggered bool
		description            string
	}{
		{
			name: "failure domain removed - should trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd2": clusterv1.FailureDomainSpec{ControlPlane: true},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
			},
			expectRolloutTriggered: true,
			description:            "fd1 was removed from cluster, machine still uses it",
		},
		{
			name: "failure domain disabled - should trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{ControlPlane: false}, // Disabled
						"fd2": clusterv1.FailureDomainSpec{ControlPlane: true},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
			},
			expectRolloutTriggered: true,
			description:            "fd1 was disabled, machine still uses it",
		},
		{
			name: "distribution improvement - should trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{ControlPlane: true},
						"fd2": clusterv1.FailureDomainSpec{ControlPlane: true},
						"fd3": clusterv1.FailureDomainSpec{ControlPlane: true},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd1"),
				createMachine("m3", "fd2"),
			},
			expectRolloutTriggered: true,
			description:            "fd3 was added, can improve from [2,1] to [1,1,1]",
		},
		{
			name: "no changes - should not trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{ControlPlane: true},
						"fd2": clusterv1.FailureDomainSpec{ControlPlane: true},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
			},
			expectRolloutTriggered: false,
			description:            "same failure domains, no changes",
		},
		{
			name: "no meaningful changes - should not trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: &clusterv1.Topology{},
					ControlPlaneRef: &corev1.ObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: clusterv1.FailureDomains{
						"fd1": clusterv1.FailureDomainSpec{ControlPlane: true},
						"fd2": clusterv1.FailureDomainSpec{ControlPlane: true},
						"fd3": clusterv1.FailureDomainSpec{ControlPlane: true},
						"fd4": clusterv1.FailureDomainSpec{ControlPlane: true}, // 4th FD added
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
				createMachine("m3", "fd3"),
			},
			expectRolloutTriggered: false,
			description:            "fd4 was added but distribution [1,1,1] is already optimal, no improvement possible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with test objects
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

			// Store initial state
			initialRolloutAfter := tt.kcp.Spec.RolloutAfter

			// Execute reconciliation
			req := reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(tt.cluster),
			}

			result, err := r.Reconcile(context.Background(), req)
			require.NoError(t, err, "Reconciliation should not fail")
			require.Equal(t, reconcile.Result{}, result, "Should return empty result")

			// Verify the KCP was updated correctly
			var updatedKCP controlplanev1.KubeadmControlPlane
			err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(tt.kcp), &updatedKCP)
			require.NoError(t, err, "Should be able to get updated KCP")

			if tt.expectRolloutTriggered {
				require.NotNil(t, updatedKCP.Spec.RolloutAfter, "RolloutAfter should be set")
				require.NotEqual(t, initialRolloutAfter, updatedKCP.Spec.RolloutAfter, "RolloutAfter should be updated")
				require.WithinDuration(
					t,
					time.Now(),
					updatedKCP.Spec.RolloutAfter.Time,
					5*time.Second,
					"RolloutAfter should be recent",
				)
			} else {
				require.Equal(t, initialRolloutAfter, updatedKCP.Spec.RolloutAfter, "RolloutAfter should not be changed")
			}
		})
	}
}

// TestReconciler_SetupWithManager tests the controller setup.
func TestReconciler_SetupWithManager(t *testing.T) {
	// This test verifies that SetupWithManager doesn't panic when called with nil
	// In real usage, this would be called with a proper manager
	r := &Reconciler{}
	options := &controller.Options{
		MaxConcurrentReconciles: 1,
	}

	// Test that calling SetupWithManager with nil manager returns an error gracefully
	// This ensures the function handles edge cases without panicking
	err := r.SetupWithManager(nil, options)
	require.Error(t, err, "SetupWithManager should return an error with nil manager")
}

// TestReconciler_kubeadmControlPlaneToCluster tests the KCP to cluster mapping.
func TestReconciler_kubeadmControlPlaneToCluster(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name            string
		obj             client.Object
		clusters        []clusterv1.Cluster
		expectedRequest []reconcile.Request
		description     string
	}{
		{
			name: "valid KCP with matching cluster",
			obj: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			clusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-namespace",
					},
					Spec: clusterv1.ClusterSpec{
						ControlPlaneRef: &corev1.ObjectReference{
							Name: "test-kcp",
						},
					},
				},
			},
			expectedRequest: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				},
			},
			description: "should return reconcile request for matching cluster",
		},
		{
			name: "valid KCP with no matching cluster",
			obj: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			clusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-cluster",
						Namespace: "test-namespace",
					},
					Spec: clusterv1.ClusterSpec{
						ControlPlaneRef: &corev1.ObjectReference{
							Name: "other-kcp",
						},
					},
				},
			},
			expectedRequest: nil,
			description:     "should return nil when no matching cluster found",
		},
		{
			name: "invalid object type",
			obj: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			clusters:        []clusterv1.Cluster{},
			expectedRequest: nil,
			description:     "should return nil for non-KCP objects",
		},
		{
			name: "cluster with nil ControlPlaneRef",
			obj: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			clusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-namespace",
					},
					Spec: clusterv1.ClusterSpec{
						ControlPlaneRef: nil,
					},
				},
			},
			expectedRequest: nil,
			description:     "should return nil when cluster has no ControlPlaneRef",
		},
		{
			name: "multiple clusters with one match",
			obj: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			clusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-cluster",
						Namespace: "test-namespace",
					},
					Spec: clusterv1.ClusterSpec{
						ControlPlaneRef: &corev1.ObjectReference{
							Name: "other-kcp",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster",
						Namespace: "test-namespace",
					},
					Spec: clusterv1.ClusterSpec{
						ControlPlaneRef: &corev1.ObjectReference{
							Name: "test-kcp",
						},
					},
				},
			},
			expectedRequest: []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: "test-namespace",
						Name:      "test-cluster",
					},
				},
			},
			description: "should return reconcile request for the matching cluster among multiple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with test clusters
			objs := make([]client.Object, 0, len(tt.clusters))
			for i := range tt.clusters {
				objs = append(objs, &tt.clusters[i])
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objs...).
				Build()

			r := &Reconciler{
				Client: fakeClient,
			}

			// Execute the mapping function
			result := r.kubeadmControlPlaneToCluster(context.Background(), tt.obj)

			// Verify the result
			require.Equal(t, tt.expectedRequest, result, "Test case: %s", tt.description)
		})
	}
}

// TestOptions_AddFlags tests the flag registration.
func TestOptions_AddFlags(t *testing.T) {
	// Test that AddFlags doesn't panic when called
	t.Run("AddFlags doesn't panic", func(t *testing.T) {
		flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
		options := &Options{}

		// This should not panic
		require.NotPanics(t, func() {
			options.AddFlags(flagSet)
		}, "AddFlags should not panic")
	})

	// Test struct field defaults
	t.Run("default struct values", func(t *testing.T) {
		options := &Options{}

		// Test that we can set the fields directly
		options.Enabled = true
		options.Concurrency = 10

		require.True(t, options.Enabled, "Enabled should be settable")
		require.Equal(t, 10, options.Concurrency, "Concurrency should be settable")
	})
}

// Helper function to create machines for testing.
func createMachine(name, failureDomain string) clusterv1.Machine {
	return clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test-namespace",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:         "test-cluster",
				clusterv1.MachineControlPlaneLabel: "",
			},
		},
		Spec: clusterv1.MachineSpec{
			FailureDomain: ptr.To(failureDomain),
		},
	}
}
