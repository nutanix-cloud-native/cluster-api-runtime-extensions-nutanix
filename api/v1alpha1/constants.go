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
	// COSIVariableName is the COSI external patch variable name.
	COSIVariableName = "cosi"
	// ClusterAutoscalerVariableName is the cluster-autoscaler external patch variable name.
	ClusterAutoscalerVariableName = "clusterAutoscaler"
	// ServiceLoadBalancerVariableName is the Service LoadBalancer config patch variable name.
	ServiceLoadBalancerVariableName = "serviceLoadBalancer"
	// RegistryAddonVariableName is the OCI registry config patch variable name.
	RegistryAddonVariableName = "registry"

	// GlobalMirrorVariableName is the global image registry mirror patch variable name.
	GlobalMirrorVariableName = "globalImageRegistryMirror"
	// ImageRegistriesVariableName is the image registries patch variable name.
	ImageRegistriesVariableName = "imageRegistries"

	// DNSVariableName is the DNS external patch variable name.
	DNSVariableName = "dns"

	ClusterUUIDAnnotationKey = APIGroup + "/cluster-uuid"

	// SkipAutoEnablingWorkloadClusterRegistry is the key of the annotation on the Cluster
	// used to skip enabling the registry addon on workload cluster.
	SkipAutoEnablingWorkloadClusterRegistry = APIGroup + "/skip-auto-enabling-workload-cluster-registry"

	// SkipSynchronizingWorkloadClusterRegistry is the key of the annotation on the Cluster
	// used to skip deploying the components that will sync OCI artifacts from the registry
	// running on the management cluster to registry running on the workload cluster.
	SkipSynchronizingWorkloadClusterRegistry = APIGroup + "/skip-synchronizing-workload-cluster-registry"

	// PreflightChecksSkipAnnotationKey is the key of the annotation on the Cluster used to skip preflight checks.
	PreflightChecksSkipAnnotationKey = "preflight.cluster.caren.nutanix.com/skip"

	// PreflightChecksSkipAllAnnotationValue is the value used in the cluster's annotations to indicate
	// that all checks are skipped.
	PreflightChecksSkipAllAnnotationValue = "all"
)
