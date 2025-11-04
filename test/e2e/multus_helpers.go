//go:build e2e

// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/v1alpha1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/pkg/handlers/lifecycle/cni"
)

type WaitForMultusToBeReadyInWorkloadClusterInput struct {
	CNI                    *v1alpha1.CNI
	WorkloadCluster        *clusterv1.Cluster
	InfrastructureProvider string
	ClusterProxy           framework.ClusterProxy
	DaemonSetIntervals     []interface{}
	HelmReleaseIntervals   []interface{}
}

// WaitForMultusToBeReadyInWorkloadCluster verifies that Multus is deployed and ready
// in the workload cluster. Multus is auto-deployed for EKS clusters when a supported
// CNI (Cilium or Calico) is configured.
//
// Example usage:
//
//	WaitForMultusToBeReadyInWorkloadCluster(
//		ctx,
//		WaitForMultusToBeReadyInWorkloadClusterInput{
//			CNI:                    addonsConfig.CNI,
//			WorkloadCluster:        workloadCluster,
//			InfrastructureProvider: "aws",
//			ClusterProxy:           proxy,
//			DaemonSetIntervals:     config.GetIntervals(flavor, "wait-daemonset"),
//			HelmReleaseIntervals:   config.GetIntervals(flavor, "wait-helmrelease"),
//		},
//	)
func WaitForMultusToBeReadyInWorkloadCluster(
	ctx context.Context,
	input WaitForMultusToBeReadyInWorkloadClusterInput, //nolint:gocritic // This hugeParam is OK in tests.
) {
	// Multus is only auto-deployed for EKS (AWS) clusters
	if input.InfrastructureProvider != "aws" {
		return
	}

	// Multus requires a supported CNI provider (Cilium or Calico)
	if input.CNI == nil {
		return
	}

	if input.CNI.Provider != v1alpha1.CNIProviderCilium && input.CNI.Provider != v1alpha1.CNIProviderCalico {
		return
	}

	// Verify cluster uses eks-quick-start ClusterClass for EKS clusters
	if input.WorkloadCluster.Labels[clusterv1.ProviderNameLabel] == "eks" {
		By("Verifying cluster uses eks-quick-start ClusterClass")
		verifyEKSClusterClass(ctx, input.WorkloadCluster, input.ClusterProxy.GetClient())
	}

	By("Waiting for Multus HelmChartProxy to be ready")
	WaitForHelmReleaseProxyReadyForCluster(
		ctx,
		WaitForHelmReleaseProxyReadyForClusterInput{
			GetLister:       input.ClusterProxy.GetClient(),
			Cluster:         input.WorkloadCluster,
			HelmReleaseName: "multus",
		},
		input.HelmReleaseIntervals...,
	)

	By("Waiting for Multus DaemonSet to be available")
	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx,
		input.WorkloadCluster.Namespace,
		input.WorkloadCluster.Name,
	).GetClient()

	WaitForDaemonSetsAvailable(ctx, WaitForDaemonSetsAvailableInput{
		Getter: workloadClusterClient,
		DaemonSet: &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multus",
				Namespace: metav1.NamespaceSystem,
			},
		},
	}, input.DaemonSetIntervals...)

	By("Verifying Multus DaemonSet is running")
	multusDaemonSet := &appsv1.DaemonSet{}
	err := workloadClusterClient.Get(
		ctx,
		types.NamespacedName{
			Name:      "multus",
			Namespace: metav1.NamespaceSystem,
		},
		multusDaemonSet,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(multusDaemonSet.Status.NumberAvailable).To(BeNumerically(">", 0))
	Expect(multusDaemonSet.Status.NumberUnavailable).To(Equal(int32(0)))

	// Verify Multus configuration (socket path, volumes, volumeMounts)
	EnsureMultusConfiguration(
		ctx,
		EnsureMultusConfigurationInput{
			CNI:             input.CNI,
			WorkloadCluster: input.WorkloadCluster,
			ClusterProxy:    input.ClusterProxy,
			MultusDaemonSet: multusDaemonSet,
		},
	)
}

type EnsureMultusConfigurationInput struct {
	CNI             *v1alpha1.CNI
	WorkloadCluster *clusterv1.Cluster
	ClusterProxy    framework.ClusterProxy
	MultusDaemonSet *appsv1.DaemonSet
}

// EnsureMultusConfiguration verifies that Multus DaemonSet has the correct
// configuration including socket path, volumes, and volumeMounts for the
// configured CNI provider.
//
// This function verifies:
//   - The readiness socket volume exists with the correct path for the CNI provider
//   - The volumeMount exists with correct mount path and read-only setting
//
// Example usage:
//
//	EnsureMultusConfiguration(
//		ctx,
//		EnsureMultusConfigurationInput{
//			CNI:             addonsConfig.CNI,
//			WorkloadCluster: workloadCluster,
//			ClusterProxy:    proxy,
//			MultusDaemonSet: multusDaemonSet,
//		},
//	)
func EnsureMultusConfiguration(
	ctx context.Context,
	input EnsureMultusConfigurationInput,
) {
	if input.CNI == nil {
		return
	}

	// Get expected socket path for the CNI provider
	expectedSocketPath, err := cni.ReadinessSocketPath(input.CNI.Provider)
	if err != nil {
		// If CNI provider doesn't support socket path, skip configuration verification
		return
	}

	By("Verifying Multus readiness socket path configuration")
	podSpec := input.MultusDaemonSet.Spec.Template.Spec

	// Verify volume exists with correct socket path
	By("Verifying Multus volume configuration")
	foundVolume := false
	for _, volume := range podSpec.Volumes {
		if volume.Name == "cni-readiness-sock" {
			foundVolume = true
			Expect(volume.HostPath).ToNot(BeNil(), "cni-readiness-sock volume should have hostPath")
			Expect(volume.HostPath.Path).To(Equal(expectedSocketPath),
				"cni-readiness-sock volume hostPath should match expected socket path")
			Expect(volume.HostPath.Type).ToNot(BeNil())
			// Verify the hostPath type is Socket (value is "Socket" string)
			Expect(*volume.HostPath.Type).To(Equal(corev1.HostPathType("Socket")),
				"cni-readiness-sock volume hostPath type should be Socket")
			break
		}
	}
	Expect(foundVolume).To(BeTrue(), "cni-readiness-sock volume should exist in Multus DaemonSet")

	// Verify volumeMount exists with correct mount path
	By("Verifying Multus volumeMount configuration")
	foundVolumeMount := false
	// Multus daemon typically runs as a container in the DaemonSet
	for _, container := range podSpec.Containers {
		for _, volumeMount := range container.VolumeMounts {
			if volumeMount.Name == "cni-readiness-sock" {
				foundVolumeMount = true
				Expect(volumeMount.MountPath).To(Equal(expectedSocketPath),
					"cni-readiness-sock volumeMount mountPath should match expected socket path")
				Expect(volumeMount.ReadOnly).To(BeTrue(),
					"cni-readiness-sock volumeMount should be read-only")
				break
			}
		}
		if foundVolumeMount {
			break
		}
	}
	Expect(foundVolumeMount).To(BeTrue(), "cni-readiness-sock volumeMount should exist in Multus container")
}

type EnsureMultusFunctionalInput struct {
	CNI             *v1alpha1.CNI
	WorkloadCluster *clusterv1.Cluster
	ClusterProxy    framework.ClusterProxy
	PodIntervals    []interface{}
}

// EnsureMultusFunctional verifies Multus functionality by creating a NetworkAttachmentDefinition
// and a test pod with Multus annotation, then verifying the secondary network interface is attached.
//
// This function:
//   - Creates a NetworkAttachmentDefinition CRD with a simple bridge CNI configuration
//   - Creates a test pod with Multus annotation to attach the secondary network
//   - Verifies the pod runs successfully with the secondary interface
//   - Cleans up test resources
//
// Example usage:
//
//	EnsureMultusFunctional(
//		ctx,
//		EnsureMultusFunctionalInput{
//			CNI:             addonsConfig.CNI,
//			WorkloadCluster: workloadCluster,
//			ClusterProxy:    proxy,
//			PodIntervals:    config.GetIntervals(flavor, "wait-pod"),
//		},
//	)
func EnsureMultusFunctional(
	ctx context.Context,
	input EnsureMultusFunctionalInput,
) {
	if input.CNI == nil {
		return
	}

	workloadClusterClient := input.ClusterProxy.GetWorkloadCluster(
		ctx,
		input.WorkloadCluster.Namespace,
		input.WorkloadCluster.Name,
	).GetClient()

	const (
		netAttachDefName = "multus-test-net-attach-def"
		testPodName      = "multus-test-pod"
		namespace        = corev1.NamespaceDefault
	)

	// Create NetworkAttachmentDefinition
	By("Creating NetworkAttachmentDefinition for Multus functional test")
	netAttachDef := createNetworkAttachmentDefinition(netAttachDefName, namespace)
	err := workloadClusterClient.Create(ctx, netAttachDef)
	Expect(err).ToNot(HaveOccurred(), "failed to create NetworkAttachmentDefinition")

	// Defer cleanup
	DeferCleanup(func() {
		By("Cleaning up NetworkAttachmentDefinition")
		_ = workloadClusterClient.Delete(ctx, netAttachDef)
	})

	// Wait for NetworkAttachmentDefinition to be available
	By("Waiting for NetworkAttachmentDefinition to be available")
	Eventually(func(g Gomega) {
		g.Expect(workloadClusterClient.Get(ctx, client.ObjectKeyFromObject(netAttachDef), netAttachDef)).To(Succeed())
	}, input.PodIntervals...).Should(Succeed(), "NetworkAttachmentDefinition was not created")

	// Create test pod with Multus annotation
	By("Creating test pod with Multus annotation")
	testPod := createMultusTestPod(testPodName, namespace, netAttachDefName)
	err = workloadClusterClient.Create(ctx, testPod)
	Expect(err).ToNot(HaveOccurred(), "failed to create test pod")

	// Defer cleanup
	DeferCleanup(func() {
		By("Cleaning up test pod")
		_ = workloadClusterClient.Delete(ctx, testPod)
	})

	// Wait for pod to be running
	By("Waiting for test pod to be running")
	Eventually(func(g Gomega) {
		g.Expect(workloadClusterClient.Get(ctx, client.ObjectKeyFromObject(testPod), testPod)).To(Succeed())
		g.Expect(testPod.Status.Phase).To(Equal(corev1.PodRunning),
			"pod should be in Running phase")
		g.Expect(testPod.Status.ContainerStatuses).ToNot(BeEmpty())
		g.Expect(testPod.Status.ContainerStatuses[0].Ready).To(BeTrue(),
			"pod container should be ready")
	}, input.PodIntervals...).Should(Succeed(), "test pod did not become ready")

	// Verify Multus annotation was processed
	By("Verifying Multus network attachment")
	networkStatusKey := "k8s.v1.cni.cncf.io/network-status"
	networkStatus, exists := testPod.Annotations[networkStatusKey]
	Expect(exists).To(BeTrue(), "pod should have network-status annotation from Multus")
	Expect(networkStatus).ToNot(BeEmpty(), "network-status annotation should not be empty")

	// Verify network-status contains our network attachment
	var networkStatusList []map[string]interface{}
	err = json.Unmarshal([]byte(networkStatus), &networkStatusList)
	Expect(err).ToNot(HaveOccurred(), "failed to parse network-status annotation")

	// Should have at least 2 networks: primary CNI + our secondary network
	Expect(len(networkStatusList)).To(BeNumerically(">=", 2),
		"pod should have at least 2 network interfaces (primary CNI + Multus secondary)")

	// Find our network attachment by name
	foundSecondaryNetwork := false
	for _, netStatus := range networkStatusList {
		if name, ok := netStatus["name"].(string); ok && name == netAttachDefName {
			foundSecondaryNetwork = true
			// Verify interface name exists
			interfaceName, ok := netStatus["interface"].(string)
			Expect(ok).To(BeTrue(), "network status should have interface name")
			Expect(interfaceName).ToNot(BeEmpty(), "interface name should not be empty")
			break
		}
	}
	Expect(foundSecondaryNetwork).To(BeTrue(),
		"pod should have attached the secondary network from NetworkAttachmentDefinition")
}

// createNetworkAttachmentDefinition creates an unstructured NetworkAttachmentDefinition object
// with a simple bridge CNI configuration for testing.
func createNetworkAttachmentDefinition(name, namespace string) *unstructured.Unstructured {
	// Simple bridge CNI config for testing
	cniConfig := map[string]interface{}{
		"cniVersion": "0.3.1",
		"name":       "test-bridge",
		"type":       "bridge",
		"bridge":     "testbr0",
		"ipam": map[string]interface{}{
			"type":   "host-local",
			"subnet": "10.10.0.0/16",
		},
	}
	cniConfigJSON, err := json.Marshal(cniConfig)
	Expect(err).ToNot(HaveOccurred(), "failed to marshal CNI config")

	netAttachDef := &unstructured.Unstructured{}
	netAttachDef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k8s.cni.cncf.io",
		Version: "v1",
		Kind:    "NetworkAttachmentDefinition",
	})
	netAttachDef.SetName(name)
	netAttachDef.SetNamespace(namespace)

	// Set spec.config with the CNI configuration
	err = unstructured.SetNestedField(netAttachDef.Object, string(cniConfigJSON), "spec", "config")
	Expect(err).ToNot(HaveOccurred(), "failed to set CNI config in NetworkAttachmentDefinition")

	return netAttachDef
}

// createMultusTestPod creates a test pod with Multus annotation to attach secondary network.
func createMultusTestPod(name, namespace, netAttachDefName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				// Multus annotation to attach the secondary network
				"k8s.v1.cni.cncf.io/networks": netAttachDefName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "registry.k8s.io/e2e-test-images/agnhost:2.57",
					Args:  []string{"pause"}, // Keep pod running
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

// verifyEKSClusterClass verifies that the cluster uses the eks-quick-start ClusterClass
// and has the correct provider label. This ensures Multus is being tested with the
// correct EKS ClusterClass configuration.
func verifyEKSClusterClass(
	ctx context.Context,
	cluster *clusterv1.Cluster,
	client client.Client,
) {
	Expect(cluster.Spec.Topology).ToNot(BeNil())
	Expect(cluster.Spec.Topology.Class).To(Equal("eks-quick-start"))
	Expect(cluster.Labels[clusterv1.ProviderNameLabel]).To(Equal("eks"))

	// Verify ClusterClass exists
	clusterClass := &clusterv1.ClusterClass{}
	err := client.Get(
		ctx,
		types.NamespacedName{
			Name: "eks-quick-start",
		},
		clusterClass,
	)
	Expect(err).ToNot(HaveOccurred())
	Expect(clusterClass.Labels[clusterv1.ProviderNameLabel]).To(Equal("eks"))
}
