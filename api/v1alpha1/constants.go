// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// ClusterConfigVariableName is the meta cluster config patch variable name.
	ClusterConfigVariableName = "clusterConfig"
	// ControlPlaneConfigVariableName is the control-plane config patch variable name.
	ControlPlaneConfigVariableName = "controlPlane"
	// WorkerConfigVariableName is the meta worker config patch variable name.
	WorkerConfigVariableName = "workerConfig"

	// AWSVariableName is the AWS config patch variable name.
	AWSVariableName = "aws"
	// DockerVariableName is the Docker config patch variable name.
	DockerVariableName = "docker"
	// NutanixVariableName is the Nutanix config patch variable name.
	NutanixVariableName = "nutanix"

	// CNIVariableName is the CNI external patch variable name.
	CNIVariableName = "cni"
	// NFDVariableName is the NFD external patch variable name.
	NFDVariableName = "nfd"

	// ClusterAutoscalerVariableName is the cluster-autoscaler external patch variable name.
	ClusterAutoscalerVariableName = "clusterAutoscaler"
	// ServiceLoadBalancerVariableName is the Service LoadBalancer config patch variable name.
	ServiceLoadBalancerVariableName = "serviceLoadBalancer"

	// GlobalMirrorVariableName is the global image registry mirror patch variable name.
	GlobalMirrorVariableName = "globalImageRegistryMirror"
	// ImageRegistriesVariableName is the image registries patch variable name.
	ImageRegistriesVariableName = "imageRegistries"

	// NamespaceSyncLabelKey is a label that can be applied to a namespace.
	//
	// When a namespace has a label with this key, ClusterClasses and their Templates are
	// copied to the namespace from a source namespace. The copies are not updated or deleted.
	NamespaceSyncLabelKey = "caren.nutanix.com/namespace-sync"
)
