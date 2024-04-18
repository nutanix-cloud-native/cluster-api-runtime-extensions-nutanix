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
	// +optional
	Provider string `json:"provider,omitempty"`
	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`
}

// NFD tells us to enable or disable the node feature discovery addon.
type NFD struct {
	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`
}

// ClusterAutoscaler tells us to enable or disable the cluster-autoscaler addon.
type ClusterAutoscaler struct {
	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`
}

type DefaultStorage struct {
	ProviderName           string `json:"providerName"`
	StorageClassConfigName string `json:"storageClassConfigName"`
}

type CSI struct {
	// +optional
	Providers []CSIProvider `json:"providers,omitempty"`
	// +optional
	DefaultStorage *DefaultStorage `json:"defaultStorage,omitempty"`
}

type CSIProvider struct {
	Name string `json:"name"`

	// +optional
	StorageClassConfig []StorageClassConfig `json:"storageClassConfig,omitempty"`

	Strategy AddonStrategy `json:"strategy"`

	// +optional
	Credentials *corev1.LocalObjectReference `json:"credentials,omitempty"`
}

type StorageClassConfig struct {
	Name string `json:"name"`

	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// +optional
	ReclaimPolicy corev1.PersistentVolumeReclaimPolicy `json:"reclaimPolicy,omitempty"`

	// +optional
	VolumeBindingMode storagev1.VolumeBindingMode `json:"volumeBindingMode,omitempty"`

	// +optional
	AllowExpansion bool `json:"allowExpansion,omitempty"`
}

// CCM tells us to enable or disable the cloud provider interface.
type CCM struct {
	// A reference to the Secret for credential information for the target Prism Central instance
	// +optional
	Credentials *corev1.LocalObjectReference `json:"credentials,omitempty"`
}
