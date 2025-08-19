// Copyright 2025 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	carenv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
)

func TestDefaultingRegistryAddon(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	_ = createTestManagementCluster(
		t,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
					Registry: &carenv1.RegistryAddon{
						Provider: carenv1.RegistryProviderCNCFDistribution,
					},
				},
			},
		},
	)
	// Create a workload cluster.
	workloadCluster := createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
				},
			},
		},
	)

	// Validate registry addon is automatically enabled in the workload cluster.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(workloadCluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterConfig.Addons.Registry).ToNot(BeNil())
	g.Expect(clusterConfig.Addons.Registry.Provider).To(Equal(carenv1.RegistryProviderCNCFDistribution))
}

func TestDefaultingShouldBeSkippedWithAnnotation(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	_ = createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
					Registry: &carenv1.RegistryAddon{
						Provider: carenv1.RegistryProviderCNCFDistribution,
					},
				},
			},
		},
	)
	// Create a workload cluster.
	workloadCluster := createTestCluster(
		t,
		map[string]string{
			carenv1.SkipAutoEnablingWorkloadClusterRegistry: "true",
		},
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
				},
			},
		},
	)

	// Validate registry addon is not automatically enabled when the skip annotation is present.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(workloadCluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterConfig.Addons.Registry).To(BeNil())
}

func TestDefaultingShouldBeSkippedWithNonTopologyCluster(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	_ = createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
					Registry: &carenv1.RegistryAddon{
						Provider: carenv1.RegistryProviderCNCFDistribution,
					},
				},
			},
		},
	)
	// Create a workload cluster.
	workloadCluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Namespace:    metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{},
	}
	g.Expect(env.Client.Create(ctx, workloadCluster)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, workloadCluster)).To(Succeed())
	})

	// Validate that the registry addon is not automatically enabled in a non-topology cluster.
	g.Expect(workloadCluster.Spec.Topology).To(BeNil())
}

func TestDefaultingShouldBeSkippedWhenRegistryAlreadyEnabled(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	_ = createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
					Registry: &carenv1.RegistryAddon{
						Provider: carenv1.RegistryProviderCNCFDistribution,
					},
				},
			},
		},
	)
	// Create a workload cluster.
	workloadCluster := createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI:      &carenv1.CNI{},
					Registry: &carenv1.RegistryAddon{},
				},
			},
		},
	)

	// Validate that the registry addon is not updated when it is already enabled in the workload cluster.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(workloadCluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterConfig.Addons.Registry).ToNot(BeNil())
	g.Expect(clusterConfig.Addons.Registry.Provider).To(BeEmpty())
}

func TestDefaultingShouldBeSkippedWhenGlobalImageRegistryMirrorIsSet(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	_ = createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
					Registry: &carenv1.RegistryAddon{
						Provider: carenv1.RegistryProviderCNCFDistribution,
					},
				},
			},
		},
	)
	// Create a workload cluster.
	workloadCluster := createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			GenericClusterConfigResource: carenv1.GenericClusterConfigResource{
				GlobalImageRegistryMirror: &carenv1.GlobalImageRegistryMirror{
					URL: "mirror.com",
				},
			},
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
				},
			},
		},
	)

	// Validate that the registry addon is not enabled when global image registry mirror is set.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(workloadCluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterConfig.Addons.Registry).To(BeNil())
}

func TestDefaultingShouldBeSkippedWhenRegistryNotEnabledInManagementCluster(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	_ = createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
				},
			},
		},
	)
	// Create a workload cluster.
	workloadCluster := createTestCluster(
		t,
		map[string]string{
			carenv1.SkipAutoEnablingWorkloadClusterRegistry: "true",
		},
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
				},
			},
		},
	)

	// Validate registry addon is not automatically enabled when the management cluster does not have it enabled.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(workloadCluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterConfig.Addons.Registry).To(BeNil())
}

func TestDefaultingShouldBeSkippedForAManagementCluster(t *testing.T) {
	g := NewWithT(t)

	// Create a management cluster.
	managementCluster := createTestCluster(
		t,
		nil,
		&carenv1.DockerClusterConfigSpec{
			Addons: &carenv1.DockerAddons{
				GenericAddons: carenv1.GenericAddons{
					CNI: &carenv1.CNI{},
				},
			},
		},
	)

	// Validate that the registry addon is not automatically enabled in a management cluster.
	clusterConfig, err := variables.UnmarshalClusterConfigVariable(managementCluster.Spec.Topology.Variables)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(clusterConfig.Addons.Registry).To(BeNil())
}

func createTestManagementCluster(
	t *testing.T,
	clusterConfigSpec *carenv1.DockerClusterConfigSpec,
) *clusterv1.Cluster {
	t.Helper()
	g := NewWithT(t)

	managementCluster := createTestCluster(
		t,
		nil,
		clusterConfigSpec,
	)

	// Create a node in the management cluster that refers to the management cluster.
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-node",
			Annotations: map[string]string{
				clusterv1.ClusterNameAnnotation:      managementCluster.Name,
				clusterv1.ClusterNamespaceAnnotation: managementCluster.Namespace,
			},
		},
	}
	g.Expect(env.Client.Create(ctx, node)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, node)).To(Succeed())
	})

	return managementCluster
}

func createTestCluster(
	t *testing.T,
	annotations map[string]string,
	clusterConfigSpec *carenv1.DockerClusterConfigSpec,
) *clusterv1.Cluster {
	t.Helper()
	g := NewWithT(t)

	variable, err := variables.MarshalToClusterVariable(carenv1.ClusterConfigVariableName, clusterConfigSpec)
	g.Expect(err).ToNot(HaveOccurred())
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-cluster-",
			Annotations:  annotations,
			Namespace:    metav1.NamespaceDefault,
		},
		Spec: clusterv1.ClusterSpec{
			Topology: &clusterv1.Topology{
				Class:   "dummy-class",
				Version: "dummy-version",
				Variables: []clusterv1.ClusterVariable{
					*variable,
				},
			},
		},
	}
	g.Expect(env.Client.Create(ctx, cluster)).To(Succeed())
	t.Cleanup(func() {
		g.Expect(env.Client.Delete(ctx, cluster)).To(Succeed())
	})

	return cluster
}
