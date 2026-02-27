// Copyright 2026 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyManagerPolicy represents the policy for the Topology Manager.
type TopologyManagerPolicy string

const (
	TopologyManagerPolicyNone           TopologyManagerPolicy = "none"
	TopologyManagerPolicyBestEffort     TopologyManagerPolicy = "best-effort"
	TopologyManagerPolicyRestricted     TopologyManagerPolicy = "restricted"
	TopologyManagerPolicySingleNUMANode TopologyManagerPolicy = "single-numa-node"
)

// CPUManagerPolicy represents the policy for the CPU Manager.
type CPUManagerPolicy string

const (
	CPUManagerPolicyNone   CPUManagerPolicy = "none"
	CPUManagerPolicyStatic CPUManagerPolicy = "static"
)

// MemoryManagerPolicy represents the policy for the Memory Manager.
type MemoryManagerPolicy string

const (
	MemoryManagerPolicyNone   MemoryManagerPolicy = "None"
	MemoryManagerPolicyStatic MemoryManagerPolicy = "Static"
)

// KubeletConfiguration defines configurable fields for the kubelet's KubeletConfiguration.
// These fields are written as a strategic merge patch file applied during kubeadm init/join.
// +kubebuilder:validation:XValidation:rule="!has(self.imageGCHighThresholdPercent) || !has(self.imageGCLowThresholdPercent) || self.imageGCHighThresholdPercent > self.imageGCLowThresholdPercent",message="imageGCHighThresholdPercent must be greater than imageGCLowThresholdPercent"
// +kubebuilder:validation:XValidation:rule="!has(self.evictionSoftGracePeriod) || has(self.evictionSoft)",message="evictionSoft must be set when evictionSoftGracePeriod is set"
// +kubebuilder:validation:XValidation:rule="!has(self.evictionSoftGracePeriod) || !has(self.evictionSoft) || self.evictionSoftGracePeriod.all(k, k in self.evictionSoft)",message="evictionSoftGracePeriod keys must match evictionSoft keys"
type KubeletConfiguration struct {
	// MaxPods defines the maximum number of pods that can run on a node.
	// Default kubelet value is 110.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4096
	MaxPods *int32 `json:"maxPods,omitempty"`

	// SystemReserved is a set of ResourceName=ResourceQuantity pairs that describe
	// resources reserved for OS system daemons. Kubernetes components such as the
	// kubelet are excluded from this reservation.
	// Valid keys: cpu, memory, ephemeral-storage, pid.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=4
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['cpu', 'memory', 'ephemeral-storage', 'pid'])",message="only cpu, memory, ephemeral-storage, and pid are valid keys"
	SystemReserved map[string]resource.Quantity `json:"systemReserved,omitempty"`

	// KubeReserved is a set of ResourceName=ResourceQuantity pairs that describe
	// resources reserved for Kubernetes system components (the kubelet, container
	// runtime, etc.).
	// Valid keys: cpu, memory, ephemeral-storage, pid.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=4
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['cpu', 'memory', 'ephemeral-storage', 'pid'])",message="only cpu, memory, ephemeral-storage, and pid are valid keys"
	KubeReserved map[string]resource.Quantity `json:"kubeReserved,omitempty"`

	// EvictionHard is a map of signal names to quantities that define hard eviction
	// thresholds. When the node reaches these thresholds, pods are evicted immediately
	// with no grace period. Values may be absolute quantities (e.g. "100Mi") or
	// percentages (e.g. "10%").
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=6
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['memory.available', 'nodefs.available', 'nodefs.inodesFree', 'imagefs.available', 'imagefs.inodesFree', 'pid.available'])",message="only valid eviction signal names are allowed"
	EvictionHard map[string]string `json:"evictionHard,omitempty"`

	// EvictionSoft is a map of signal names to quantities that define soft eviction
	// thresholds. Pods are evicted after the grace period specified in
	// evictionSoftGracePeriod has elapsed. Values may be absolute quantities or
	// percentages.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=6
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['memory.available', 'nodefs.available', 'nodefs.inodesFree', 'imagefs.available', 'imagefs.inodesFree', 'pid.available'])",message="only valid eviction signal names are allowed"
	EvictionSoft map[string]string `json:"evictionSoft,omitempty"`

	// EvictionSoftGracePeriod is a map of signal names to durations that define
	// grace periods for soft eviction signals. Each key must correspond to an entry
	// in evictionSoft.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxProperties=6
	// +kubebuilder:validation:XValidation:rule="self.all(k, k in ['memory.available', 'nodefs.available', 'nodefs.inodesFree', 'imagefs.available', 'imagefs.inodesFree', 'pid.available'])",message="only valid eviction signal names are allowed"
	EvictionSoftGracePeriod map[string]metav1.Duration `json:"evictionSoftGracePeriod,omitempty"`

	// ProtectKernelDefaults when enabled causes the kubelet to error if kernel flags
	// are not as it expects. Typically required by CIS benchmarks and DISA STIG.
	// +kubebuilder:validation:Optional
	ProtectKernelDefaults *bool `json:"protectKernelDefaults,omitempty"`

	// TopologyManagerPolicy controls the NUMA-aware resource alignment policy.
	// Relevant for workloads sensitive to hardware topology (GPU, HPC, telco).
	// Default kubelet value is "none".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=none;best-effort;restricted;single-numa-node
	TopologyManagerPolicy *TopologyManagerPolicy `json:"topologyManagerPolicy,omitempty"`

	// CPUManagerPolicy controls how cpusets are assigned to containers.
	// "static" enables exclusive CPU pinning for Guaranteed QoS pods.
	// Default kubelet value is "none".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=none;static
	CPUManagerPolicy *CPUManagerPolicy `json:"cpuManagerPolicy,omitempty"`

	// MemoryManagerPolicy controls the memory management policy on the node.
	// "Static" enables NUMA-aware memory allocation for Guaranteed QoS pods.
	// Default kubelet value is "None".
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=None;Static
	MemoryManagerPolicy *MemoryManagerPolicy `json:"memoryManagerPolicy,omitempty"`

	// PodPidsLimit is the maximum number of PIDs in any pod.
	// Prevents fork bombs and enforces per-pod PID limits.
	// Default kubelet value is -1 (unlimited).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=-1
	PodPidsLimit *int64 `json:"podPidsLimit,omitempty"`

	// ContainerLogMaxSize defines the maximum size of the container log file
	// before it is rotated. Value is a quantity (e.g. "10Mi", "256Ki").
	// Default kubelet value is "10Mi".
	// +kubebuilder:validation:Optional
	ContainerLogMaxSize *resource.Quantity `json:"containerLogMaxSize,omitempty"`

	// ContainerLogMaxFiles specifies the maximum number of container log files
	// that can be present for a container.
	// Default kubelet value is 5.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=2
	ContainerLogMaxFiles *int32 `json:"containerLogMaxFiles,omitempty"`

	// ImageGCHighThresholdPercent is the percent of disk usage after which image
	// garbage collection is always run. Must be greater than
	// imageGCLowThresholdPercent when both are set.
	// Default kubelet value is 85.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	ImageGCHighThresholdPercent *int32 `json:"imageGCHighThresholdPercent,omitempty"`

	// ImageGCLowThresholdPercent is the percent of disk usage before which image
	// garbage collection is never run. Must be less than
	// imageGCHighThresholdPercent when both are set.
	// Default kubelet value is 80.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	ImageGCLowThresholdPercent *int32 `json:"imageGCLowThresholdPercent,omitempty"`

	// MaxParallelImagePulls defines the maximum number of image pulls performed
	// in parallel by the kubelet. A value of zero means unlimited. When set to a
	// value > 0, serializeImagePulls is automatically set to false.
	// Default kubelet value is nil (serial pulls).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=0
	MaxParallelImagePulls *int32 `json:"maxParallelImagePulls,omitempty"`

	// ShutdownGracePeriod specifies the total duration that the node should delay
	// the shutdown by for pod termination during a node shutdown.
	// Default kubelet value is "0s" (disabled).
	// +kubebuilder:validation:Optional
	ShutdownGracePeriod *metav1.Duration `json:"shutdownGracePeriod,omitempty"`

	// ShutdownGracePeriodCriticalPods specifies the duration used to terminate
	// critical pods during a node shutdown. This should be less than
	// shutdownGracePeriod.
	// Default kubelet value is "0s".
	// +kubebuilder:validation:Optional
	ShutdownGracePeriodCriticalPods *metav1.Duration `json:"shutdownGracePeriodCriticalPods,omitempty"`
}
