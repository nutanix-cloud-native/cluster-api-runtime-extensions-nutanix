// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"testing"

	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestGenerateNoProxy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cluster         *clusterv1.Cluster
		expectedNoProxy []string
		additonalNo     []string
	}{{
		name:    "no networking config",
		cluster: &clusterv1.Cluster{},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name:        "no networking config with additional no proxy",
		cluster:     &clusterv1.Cluster{},
		additonalNo: []string{"example.com"},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.", "example.com",
		},
	}, {
		name: "custom pod network",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Pods: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"10.0.0.0/24", "10.0.1.0/24"},
					},
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "10.0.0.0/24", "10.0.1.0/24", "kubernetes",
			"kubernetes.default", ".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name: "Unknown infrastructure cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "SomeFakeInfrastructureCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name: "AWS cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AWSCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.", "169.254.169.254", ".elb.amazonaws.com",
		},
	}, {
		name: "AWS managed (EKS) cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AWSManagedCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.", "169.254.169.254", ".elb.amazonaws.com",
		},
	}, {
		name: "Azure cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AzureCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.", "169.254.169.254",
		},
	}, {
		name: "Azure managed (AKS) cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "AzureCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.", "169.254.169.254",
		},
	}, {
		name: "GCP cluster",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &v1.ObjectReference{
					Kind: "GCPCluster",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
			"169.254.169.254", "metadata", "metadata.google.internal",
		},
	}, {
		name: "custom service network",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Services: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/24", "172.16.1.0/24"},
					},
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "172.16.0.0/24", "172.16.1.0/24", "kubernetes",
			"kubernetes.default", ".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name: "custom servicedomain",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					ServiceDomain: "foo.bar",
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.foo.bar", ".svc.foo.bar.",
		},
	}, {
		name: "all options",
		cluster: &clusterv1.Cluster{
			Spec: clusterv1.ClusterSpec{
				ClusterNetwork: &clusterv1.ClusterNetwork{
					Pods: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"10.10.0.0/16"},
					},
					Services: &clusterv1.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/16"},
					},
					ServiceDomain: "foo.bar",
				},
			},
		},
		additonalNo: []string{"example.com"},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "10.10.0.0/16", "172.16.0.0/16", "kubernetes",
			"kubernetes.default", ".svc", ".svc.foo.bar", ".svc.foo.bar.", "example.com",
		},
	}}

	for idx := range testCases {
		tt := testCases[idx]

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			g.Expect((&v1alpha1.HTTPProxy{
				AdditionalNo: tt.additonalNo,
			}).GenerateNoProxy(tt.cluster)).To(gomega.Equal(tt.expectedNoProxy))
		})
	}
}
