// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package failuredomainrollout

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
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
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
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
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
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
						FailureDomain: "fd1", // Using fd1 which is now removed
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
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(false), // Disabled for control plane
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
						FailureDomain: "fd1", // Using fd1 which is now disabled
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
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
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
						FailureDomain: "fd1", // Using fd1 which is still available
					},
				},
			},
			expectedRollout: false,
		},
		{
			name: "new failure domain improves distribution - should trigger rollout",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
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
						FailureDomain: "fd1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-2",
						Namespace: "test-namespace",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         "test-cluster",
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: clusterv1.MachineSpec{
						FailureDomain: "fd1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-machine-3",
						Namespace: "test-namespace",
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         "test-cluster",
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: clusterv1.MachineSpec{
						FailureDomain: "fd1",
					},
				},
			},
			expectedRollout: true,
			expectedReason:  "failure domain distribution could be improved for better fault tolerance",
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

		{
			name:                    "no available failure domains",
			replicas:                0,
			machines:                []clusterv1.Machine{},
			availableFailureDomains: []string{},
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

func TestReconciler_canImproveWithMoreFDs(t *testing.T) {
	r := &Reconciler{}

	tests := []struct {
		name                string
		currentDistribution map[string]int
		replicas            int
		availableCount      int
		expectedImprove     bool
		description         string
	}{
		{
			name:                "empty distribution",
			currentDistribution: map[string]int{},
			replicas:            3,
			availableCount:      3,
			expectedImprove:     false,
			description:         "empty distribution should not improve",
		},
		{
			name: "zero replicas",
			currentDistribution: map[string]int{
				"fd1": 0,
			},
			replicas:        0,
			availableCount:  2,
			expectedImprove: false,
			description:     "zero replicas cannot improve",
		},
		{
			name: "no additional FDs available",
			currentDistribution: map[string]int{
				"fd1": 2,
				"fd2": 1,
			},
			replicas:        3,
			availableCount:  2,
			expectedImprove: false,
			description:     "cannot improve if no additional FDs available",
		},
		{
			name: "1 replica on 1 FD, cannot improve with more FDs",
			currentDistribution: map[string]int{
				"fd1": 1,
			},
			replicas:        1,
			availableCount:  3,
			expectedImprove: false,
			description:     "single replica already optimal - currentMax=1, optimalMax=1",
		},
		{
			name: "3 replicas concentrated on 1 FD, can spread across 2 FDs",
			currentDistribution: map[string]int{
				"fd1": 3,
			},
			replicas:        3,
			availableCount:  2,
			expectedImprove: true,
			description:     "can improve [3] → [2,1] - currentMax=3, optimalMax=2",
		},
		{
			name: "3 replicas suboptimal [2,1], can spread across 3 FDs",
			currentDistribution: map[string]int{
				"fd1": 2,
				"fd2": 1,
			},
			replicas:        3,
			availableCount:  3,
			expectedImprove: true,
			description:     "can improve [2,1] → [1,1,1] - currentMax=2, optimalMax=1",
		},
		{
			name: "3 replicas optimally distributed across 3 FDs",
			currentDistribution: map[string]int{
				"fd1": 1,
				"fd2": 1,
				"fd3": 1,
			},
			replicas:        3,
			availableCount:  3,
			expectedImprove: false,
			description:     "already optimal - currentMax=1, optimalMax=1",
		},
		{
			name: "3 replicas optimal, extra FDs available",
			currentDistribution: map[string]int{
				"fd1": 1,
				"fd2": 1,
				"fd3": 1,
			},
			replicas:        3,
			availableCount:  5,
			expectedImprove: false,
			description:     "no improvement possible - currentMax=1, optimalMax=1",
		},
		{
			name: "5 replicas concentrated, can spread across more FDs",
			currentDistribution: map[string]int{
				"fd1": 3,
				"fd2": 2,
			},
			replicas:        5,
			availableCount:  5,
			expectedImprove: true,
			description:     "can improve [3,2] → [1,1,1,1,1] - currentMax=3, optimalMax=1",
		},
		{
			name: "5 replicas optimally distributed across 3 FDs",
			currentDistribution: map[string]int{
				"fd1": 2,
				"fd2": 2,
				"fd3": 1,
			},
			replicas:        5,
			availableCount:  3,
			expectedImprove: false,
			description:     "already optimal for 3 FDs - currentMax=2, optimalMax=2",
		},
		{
			name: "5 replicas can improve distribution quality",
			currentDistribution: map[string]int{
				"fd1": 2,
				"fd2": 2,
				"fd3": 1,
			},
			replicas:        5,
			availableCount:  5,
			expectedImprove: true,
			description:     "can improve [2,2,1] → [1,1,1,1,1] - currentMax=2, optimalMax=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.canImproveWithMoreFDs(tt.currentDistribution, tt.replicas, tt.availableCount)
			require.Equal(t, tt.expectedImprove, result, "Test case: %s", tt.description)
		})
	}
}

func TestReconciler_shouldSkipClusterReconciliation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name         string
		cluster      *clusterv1.Cluster
		expectedSkip bool
		description  string
	}{
		{
			name: "cluster without topology - should skip",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					// No Topology set
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			expectedSkip: true,
			description:  "should skip when cluster has no topology",
		},
		{
			name: "cluster without control plane ref - should skip",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					// No ControlPlaneRef set
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			expectedSkip: true,
			description:  "should skip when cluster has no control plane reference",
		},
		{
			name: "cluster without failure domains - should skip",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					// No FailureDomains set
				},
			},
			expectedSkip: true,
			description:  "should skip when cluster has no failure domains",
		},
		{
			name: "cluster with all required fields - should not skip",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(false),
						},
					},
				},
			},
			expectedSkip: false,
			description:  "should not skip when cluster has all required fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{}
			logger := logr.Discard()

			shouldSkip := r.shouldSkipClusterReconciliation(tt.cluster, logger)

			require.Equal(t, tt.expectedSkip, shouldSkip, "Test case: %s", tt.description)
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
					Conditions: clusterv1beta1.Conditions{
						{
							Type:   clusterv1beta1.ConditionType(controlplanev1.MachinesSpecUpToDateCondition),
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
		expectRequeue          bool
		expectedRequeueAfter   time.Duration
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
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
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
			expectRequeue:          false,
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
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(false), // Disabled
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
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
			expectRequeue:          false,
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
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd3",
							ControlPlane: ptr.To(true),
						},
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
			expectRequeue:          false,
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
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
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
			expectRequeue:          false,
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
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd3",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd4",
							ControlPlane: ptr.To(true), // 4th FD added
						},
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
			expectRequeue:          false,
			description:            "fd4 was added but distribution [1,1,1] is already optimal, no improvement possible",
		},
		{
			name: "kubeadmcontrolplane not reconciled - should requeue",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 7, // Higher than observedGeneration
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 4, // Lower than generation
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
			},
			expectRolloutTriggered: false,
			expectRequeue:          true,
			expectedRequeueAfter:   2 * time.Minute,
			description:            "kcp observedGeneration < generation, should requeue",
		},
		{
			name: "cluster not reconciled - should requeue",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5, // Higher than observedGeneration
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 2, // Lower than generation
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 3,
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 3, // Same as generation - KCP is reconciled
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
			},
			expectRolloutTriggered: false,
			expectRequeue:          true,
			expectedRequeueAfter:   2 * time.Minute,
			description:            "cluster observedGeneration < generation, should requeue",
		},
		{
			name: "both cluster and kcp not reconciled - should requeue",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5, // Higher than observedGeneration
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 2, // Lower than generation
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
						{
							Name:         "fd2",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 7, // Higher than observedGeneration
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 4, // Lower than generation
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
				createMachine("m2", "fd2"),
			},
			expectRolloutTriggered: false,
			expectRequeue:          true,
			expectedRequeueAfter:   2 * time.Minute,
			description:            "both cluster and kcp observedGeneration < generation, should requeue",
		},
		{
			name: "cluster being deleted - should skip reconciliation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
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
			},
			expectRolloutTriggered: false,
			expectRequeue:          false,
			description:            "cluster has deletion timestamp, should skip reconciliation",
		},
		{
			name: "kcp being deleted - should skip reconciliation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-kcp",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
			},
			expectRolloutTriggered: false,
			expectRequeue:          false,
			description:            "kcp has deletion timestamp, should skip reconciliation",
		},
		{
			name: "both cluster and kcp being deleted - should skip reconciliation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
				Spec: clusterv1.ClusterSpec{
					Topology: clusterv1.Topology{},
					ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
						Name: "test-kcp",
					},
				},
				Status: clusterv1.ClusterStatus{
					FailureDomains: []clusterv1.FailureDomain{
						{
							Name:         "fd1",
							ControlPlane: ptr.To(true),
						},
					},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-kcp",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
				Spec: controlplanev1.KubeadmControlPlaneSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			machines: []clusterv1.Machine{
				createMachine("m1", "fd1"),
			},
			expectRolloutTriggered: false,
			expectRequeue:          false,
			description:            "both cluster and kcp have deletion timestamps, should skip reconciliation",
		},
		{
			name: "cluster paused - should skip reconciliation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(true),
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expectRolloutTriggered: false,
			expectRequeue:          false,
			description:            "cluster is paused, should skip reconciliation",
		},
		{
			name: "kcp paused via annotation - should skip reconciliation",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(false),
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"cluster.x-k8s.io/paused": "",
					},
				},
			},
			expectRolloutTriggered: false,
			expectRequeue:          false,
			description:            "kcp has paused annotation, should skip reconciliation",
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

			// Check for requeue scenarios
			if tt.expectRequeue {
				require.Equal(
					t,
					reconcile.Result{RequeueAfter: tt.expectedRequeueAfter},
					result,
					"Should return requeue result",
				)

				// Verify the KCP was NOT updated in requeue scenarios
				var updatedKCP controlplanev1.KubeadmControlPlane
				err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(tt.kcp), &updatedKCP)
				require.NoError(t, err, "Should be able to get KCP")
				require.Equal(
					t,
					initialRolloutAfter,
					updatedKCP.Spec.RolloutAfter,
					"RolloutAfter should not be changed in requeue scenarios",
				)
			} else {
				require.Equal(t, reconcile.Result{}, result, "Should return empty result")

				// Verify the KCP was updated correctly for normal scenarios
				var updatedKCP controlplanev1.KubeadmControlPlane
				err = fakeClient.Get(context.Background(), client.ObjectKeyFromObject(tt.kcp), &updatedKCP)
				require.NoError(t, err, "Should be able to get updated KCP")

				if tt.expectRolloutTriggered {
					require.NotNil(t, updatedKCP.Spec.RolloutAfter, "RolloutAfter should be set")
					require.NotEqual(
						t,
						initialRolloutAfter,
						updatedKCP.Spec.RolloutAfter,
						"RolloutAfter should be updated",
					)
					require.WithinDuration(
						t,
						time.Now(),
						updatedKCP.Spec.RolloutAfter.Time,
						5*time.Second,
						"RolloutAfter should be recent",
					)
				} else {
					require.Equal(
						t,
						initialRolloutAfter,
						updatedKCP.Spec.RolloutAfter,
						"RolloutAfter should not be changed",
					)
				}
			}
		})
	}
}

// TestReconciler_areResourcesDeleting tests the deletion check helper function.
func TestReconciler_areResourcesDeleting(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *clusterv1.Cluster
		kcp      *controlplanev1.KubeadmControlPlane
		expected bool
	}{
		{
			name: "no deletion timestamps - should not be deleting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expected: false,
		},
		{
			name: "cluster has deletion timestamp - should be deleting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expected: true,
		},
		{
			name: "kcp has deletion timestamp - should be deleting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-kcp",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
			},
			expected: true,
		},
		{
			name: "both have deletion timestamps - should be deleting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-kcp",
					Namespace:         "test-namespace",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{"test-finalizer"},
				},
			},
			expected: true,
		},
		{
			name:    "nil cluster - should not be deleting",
			cluster: nil,
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expected: false,
		},
		{
			name: "nil kcp - should not be deleting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
			},
			kcp:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{}
			result := r.areResourcesDeleting(tt.cluster, tt.kcp)
			require.Equal(t, tt.expected, result, "areResourcesDeleting result should match expected")
		})
	}
}

// TestReconciler_areResourcesUpdating tests the updating check helper function.
func TestReconciler_areResourcesUpdating(t *testing.T) {
	tests := []struct {
		name     string
		cluster  *clusterv1.Cluster
		kcp      *controlplanev1.KubeadmControlPlane
		expected bool
	}{
		{
			name: "both resources reconciled - should not be updating",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5,
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 5,
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 3,
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 3,
				},
			},
			expected: false,
		},
		{
			name: "cluster not reconciled - should be updating",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5,
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 3, // Lower than generation
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 3,
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 3,
				},
			},
			expected: true,
		},
		{
			name: "kcp not reconciled - should be updating",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5,
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 5,
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 7,
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 4, // Lower than generation
				},
			},
			expected: true,
		},
		{
			name: "both not reconciled - should be updating",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5,
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 2, // Lower than generation
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 7,
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 4, // Lower than generation
				},
			},
			expected: true,
		},
		{
			name:    "nil cluster - should not be updating",
			cluster: nil,
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-kcp",
					Namespace:  "test-namespace",
					Generation: 3,
				},
				Status: controlplanev1.KubeadmControlPlaneStatus{
					ObservedGeneration: 3,
				},
			},
			expected: false,
		},
		{
			name: "nil kcp - should not be updating",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-cluster",
					Namespace:  "test-namespace",
					Generation: 5,
				},
				Status: clusterv1.ClusterStatus{
					ObservedGeneration: 5,
				},
			},
			kcp:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{}
			result := r.areResourcesUpdating(tt.cluster, tt.kcp)
			require.Equal(t, tt.expected, result, "areResourcesUpdating result should match expected")
		})
	}
}

func TestReconciler_areResourcesPaused(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterv1.AddToScheme(scheme))
	require.NoError(t, controlplanev1.AddToScheme(scheme))

	tests := []struct {
		name           string
		cluster        *clusterv1.Cluster
		kcp            *controlplanev1.KubeadmControlPlane
		expectedPaused bool
		description    string
	}{
		{
			name: "cluster not paused - should not be paused",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					// Paused field not set (defaults to false)
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expectedPaused: false,
			description:    "should not be paused when cluster is not paused",
		},
		{
			name: "cluster explicitly not paused - should not be paused",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(false),
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expectedPaused: false,
			description:    "should not be paused when cluster is explicitly not paused",
		},
		{
			name: "cluster paused - should be paused",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(true),
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expectedPaused: true,
			description:    "should be paused when cluster is paused",
		},
		{
			name:    "nil cluster - should not be paused",
			cluster: nil,
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
				},
			},
			expectedPaused: false,
			description:    "should not be paused when cluster is nil",
		},
		{
			name: "nil kcp - should respect cluster paused state",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(true),
				},
			},
			kcp:            nil,
			expectedPaused: true,
			description:    "should be paused when cluster is paused even if kcp is nil",
		},
		{
			name:           "both nil - should not be paused",
			cluster:        nil,
			kcp:            nil,
			expectedPaused: false,
			description:    "should not be paused when both cluster and kcp are nil",
		},
		{
			name: "kcp with paused annotation - should be paused",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clusterv1.ClusterSpec{
					Paused: ptr.To(false),
				},
			},
			kcp: &controlplanev1.KubeadmControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kcp",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						"cluster.x-k8s.io/paused": "",
					},
				},
			},
			expectedPaused: true,
			description:    "should be paused when kcp has paused annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{}

			isPaused := r.areResourcesPaused(tt.cluster, tt.kcp)

			require.Equal(t, tt.expectedPaused, isPaused, "Test case: %s", tt.description)
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
			FailureDomain: failureDomain,
		},
	}
}
