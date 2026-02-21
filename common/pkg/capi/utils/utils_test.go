// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestManagementOrFutureManagementCluster(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		initialClusters []clusterv1.Cluster
		nodes           []corev1.Node
		wantCluster     *clusterv1.Cluster
		wantErr         error
	}{
		{
			name: "management cluster from Node annotations",
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "default",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Annotations: map[string]string{
							clusterv1.ClusterNameAnnotation:      "management-cluster",
							clusterv1.ClusterNamespaceAnnotation: "default",
						},
					},
				},
			},
			wantCluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "management-cluster",
					Namespace: "default",
				},
			},
		},
		{
			name: "management cluster from Node annotations with multiple clusters",
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "workload-cluster",
						Namespace: "default",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Annotations: map[string]string{
							clusterv1.ClusterNameAnnotation:      "management-cluster",
							clusterv1.ClusterNamespaceAnnotation: "default",
						},
					},
				},
			},
			wantCluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "management-cluster",
					Namespace: "default",
				},
			},
		},
		{
			name: "management cluster from bootstrap client with single cluster",
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "default",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
			},
			wantCluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "management-cluster",
					Namespace: "default",
				},
			},
		},
		{
			name: "fail on missing cluster",
			// Cluster is in the wrong namespace.
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "different-namespace",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Annotations: map[string]string{
							clusterv1.ClusterNameAnnotation:      "management-cluster",
							clusterv1.ClusterNamespaceAnnotation: "default",
						},
					},
				},
			},
			wantErr: fmt.Errorf(
				`error determining management cluster for the provided client: error getting Cluster object based on Node annotations: clusters.cluster.x-k8s.io "management-cluster" not found`, //nolint:lll // Long error message.
			),
		},
		{
			name: "fail on missing cluster name annotation",
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "default",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Annotations: map[string]string{
							clusterv1.ClusterNamespaceAnnotation: "default",
						},
					},
				},
			},
			wantErr: fmt.Errorf(
				`error determining management cluster for the provided client: missing "cluster.x-k8s.io/cluster-name" annotation`, //nolint:lll // Long error message.
			),
		},
		{
			name: "fail on missing cluster namespace annotation",
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "default",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
						Annotations: map[string]string{
							clusterv1.ClusterNameAnnotation: "management-cluster",
						},
					},
				},
			},
			wantErr: fmt.Errorf(
				`error determining management cluster for the provided client: missing "cluster.x-k8s.io/cluster-namespace" annotation`, //nolint:lll // Long error message.
			),
		},
		{
			name: "fail when multiple clusters exist in bootstrap client",
			initialClusters: []clusterv1.Cluster{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "management-cluster",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-management-cluster",
						Namespace: "default",
					},
				},
			},
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
			},
			wantErr: fmt.Errorf(
				"error determining management cluster for the provided client: multiple Cluster objects found, expected exactly one", //nolint:lll // Long error message.
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cl := buildFakeClientForTest(t, tt.initialClusters, tt.nodes)

			cluster, err := ManagementOrFutureManagementCluster(context.Background(), cl)
			if tt.wantErr != nil {
				require.EqualError(t, err, tt.wantErr.Error())
			} else {
				require.NoError(t, err)
			}
			if tt.wantCluster != nil {
				assert.Equal(t, tt.wantCluster.GetName(), cluster.GetName())
				assert.Equal(t, tt.wantCluster.GetNamespace(), cluster.GetNamespace())
			}
		})
	}
}

func buildFakeClientForTest(t *testing.T, clusters []clusterv1.Cluster, nodes []corev1.Node) client.Client {
	t.Helper()
	objs := make([]client.Object, 0, len(clusters)+len(nodes))
	for i := range clusters {
		objs = append(objs, &clusters[i])
	}
	for i := range nodes {
		objs = append(objs, &nodes[i])
	}
	return buildFakeClient(t, objs...)
}

func buildFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	clientScheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(clientScheme))
	utilruntime.Must(clusterv1.AddToScheme(clientScheme))
	return fake.NewClientBuilder().WithScheme(clientScheme).WithObjects(objs...).Build()
}
