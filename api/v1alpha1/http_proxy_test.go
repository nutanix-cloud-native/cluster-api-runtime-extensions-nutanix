// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"testing"

	"github.com/onsi/gomega"
	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
)

func TestGenerateNoProxy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cluster         *clusterv1beta2.Cluster
		expectedNoProxy []string
		additonalNo     []string
	}{{
		name:    "no networking config",
		cluster: &clusterv1beta2.Cluster{},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name:        "no networking config with additional no proxy",
		cluster:     &clusterv1beta2.Cluster{},
		additonalNo: []string{"example.com"},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.", "example.com",
		},
	}, {
		name: "custom pod network",
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				ClusterNetwork: clusterv1beta2.ClusterNetwork{
					Pods: clusterv1beta2.NetworkRanges{
						CIDRBlocks: []string{"10.0.0.0/24", "10.0.1.0/24"},
					},
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1",
			"10.0.0.0/24",
			"10.0.1.0/24",
			"kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name: "Unknown infrastructure cluster",
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				InfrastructureRef: clusterv1beta2.ContractVersionedObjectReference{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				InfrastructureRef: clusterv1beta2.ContractVersionedObjectReference{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				InfrastructureRef: clusterv1beta2.ContractVersionedObjectReference{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				InfrastructureRef: clusterv1beta2.ContractVersionedObjectReference{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				InfrastructureRef: clusterv1beta2.ContractVersionedObjectReference{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				InfrastructureRef: clusterv1beta2.ContractVersionedObjectReference{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				ClusterNetwork: clusterv1beta2.ClusterNetwork{
					Services: clusterv1beta2.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/24", "172.16.1.0/24"},
					},
				},
			},
		},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1",
			"172.16.0.0/24",
			"172.16.1.0/24",
			"kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name: "custom servicedomain",
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				ClusterNetwork: clusterv1beta2.ClusterNetwork{
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
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				ClusterNetwork: clusterv1beta2.ClusterNetwork{
					Pods: clusterv1beta2.NetworkRanges{
						CIDRBlocks: []string{"10.10.0.0/16"},
					},
					Services: clusterv1beta2.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/16"},
					},
					ServiceDomain: "foo.bar",
				},
			},
		},
		additonalNo: []string{"example.com"},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1",
			"10.10.0.0/16",
			"172.16.0.0/16",
			"kubernetes", "kubernetes.default",
			".svc", ".svc.foo.bar", ".svc.foo.bar.",
			"example.com",
		},
	}}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			g.Expect((&v1alpha1.HTTPProxy{
				AdditionalNo: tt.additonalNo,
			}).GenerateNoProxy(tt.cluster)).To(gomega.Equal(tt.expectedNoProxy))
		})
	}
}

func TestGenerateNoProxyNormalized(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		cluster         *clusterv1beta2.Cluster
		expectedNoProxy []string
		additonalNo     []string
	}{{
		name:    "no networking config",
		cluster: &clusterv1beta2.Cluster{},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1", "kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
		},
	}, {
		name: "CIDRs are expanded to include IP ranges",
		cluster: &clusterv1beta2.Cluster{
			Spec: clusterv1beta2.ClusterSpec{
				ClusterNetwork: clusterv1beta2.ClusterNetwork{
					Pods: clusterv1beta2.NetworkRanges{
						CIDRBlocks: []string{"10.0.0.0/24"},
					},
					Services: clusterv1beta2.NetworkRanges{
						CIDRBlocks: []string{"172.16.0.0/16"},
					},
				},
			},
		},
		additonalNo: []string{"example.com"},
		expectedNoProxy: []string{
			"localhost", "127.0.0.1",
			"10.0.0.0/24", "10.0.0.0-10.0.0.255",
			"172.16.0.0/16", "172.16.0.0-172.16.255.255",
			"kubernetes", "kubernetes.default",
			".svc", ".svc.cluster.local", ".svc.cluster.local.",
			"example.com",
		},
	}}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := gomega.NewWithT(t)

			g.Expect((&v1alpha1.HTTPProxy{
				AdditionalNo: tt.additonalNo,
			}).GenerateNoProxyNormalized(tt.cluster)).To(gomega.Equal(tt.expectedNoProxy))
		})
	}
}
