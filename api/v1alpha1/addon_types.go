// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

const (
	AddonStrategyClusterResourceSet   AddonStrategy = "ClusterResourceSet"
	AddonStrategyHelmAddon            AddonStrategy = "HelmAddon"
	VolumeBindingImmediate                          = storagev1.VolumeBindingImmediate
	VolumeBindingWaitForFirstConsumer               = storagev1.VolumeBindingWaitForFirstConsumer

	VolumeReclaimRecycle = corev1.PersistentVolumeReclaimRecycle
	VolumeReclaimDelete  = corev1.PersistentVolumeReclaimDelete
	VolumeReclaimRetain  = corev1.PersistentVolumeReclaimRetain
)

type Addons struct {
	// +optional
	CNI *CNI `json:"cni,omitempty"`

	// +optional
	NFD *NFD `json:"nfd,omitempty"`

	// +optional
	ClusterAutoscaler *ClusterAutoscaler `json:"clusterAutoscaler,omitempty"`

	// +optional
	CCM *CCM `json:"ccm,omitempty"`

	// +optional
	CSIProviders *CSI `json:"csi,omitempty"`
}

type AddonStrategy string

// CNI required for providing CNI configuration.
type CNI struct {
	// CNI provider to deploy.
	// +kubebuilder:validation:Enum=Calico;Cilium
	Provider string `json:"provider"`
	// Addon strategy used to deploy the CNI provider to the workload cluster.
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

// NFD tells us to enable or disable the node feature discovery addon.
type NFD struct {
	// Addon strategy used to deploy Node Feature Discovery (NFD) to the workload cluster.
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

// ClusterAutoscaler tells us to enable or disable the cluster-autoscaler addon.
type ClusterAutoscaler struct {
	// Addon strategy used to deploy cluster-autoscaler to the management cluster
	// targeting the workload cluster.
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

type DefaultStorage struct {
	// Name of the CSI Provider for the default storage class.
	// +kubebuilder:validation:Enum=aws-ebs;nutanix
	ProviderName string `json:"providerName"`
	// Name of storage class config in any of the provider objects.
	StorageClassConfigName string `json:"storageClassConfigName"`
}

type CSI struct {
	// +optional
	Providers []CSIProvider `json:"providers,omitempty"`
	// +optional
	DefaultStorage *DefaultStorage `json:"defaultStorage,omitempty"`
}

type CSIProvider struct {
	// Name of the CSI Provider.
	// +kubebuilder:validation:Enum=aws-ebs;nutanix
	Name string `json:"name"`

	// +optional
	StorageClassConfig []StorageClassConfig `json:"storageClassConfig,omitempty"`

	// Addon strategy used to deploy the CSI provider to the workload cluster.
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`

	// The reference to any secret used by the CSI Provider.
	// +optional
	Credentials *corev1.LocalObjectReference `json:"credentials,omitempty"`
}

type StorageClassConfig struct {
	// Name of storage class config.
	Name string `json:"name"`

	// Parameters passed into the storage class object.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// +kubebuilder:validation:Enum=Delete;Retain;Recycle
	// +kubebuilder:default=Delete
	// +optional
	ReclaimPolicy corev1.PersistentVolumeReclaimPolicy `json:"reclaimPolicy,omitempty"`

	// +kubebuilder:validation:Enum=Immediate;WaitForFirstConsumer
	// +kubebuilder:default=WaitForFirstConsumer
	// +optional
	VolumeBindingMode storagev1.VolumeBindingMode `json:"volumeBindingMode,omitempty"`

	// If the storage class should allow volume expanding
	// +kubebuilder:default=false
	// +optional
	AllowExpansion bool `json:"allowExpansion,omitempty"`
}

// CCM tells us to enable or disable the cloud provider interface.
type CCM struct {
	// A reference to the Secret for credential information for the target Prism Central instance
	// +optional
	Credentials *corev1.LocalObjectReference `json:"credentials,omitempty"`
}
