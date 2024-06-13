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
	// CarenNamespace is the namespace that caren is deployed in.
	CarenNamespace = "caren-system"
)
