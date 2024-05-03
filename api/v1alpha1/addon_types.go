// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"

	nutanixv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

// All kubebuilder "Enum" build tag values are available in the OpenAPI spec.
// So that all these values are available to users of the api package, we
// we define a constant for each of the values.
//
// TODO: Generate these constants from the kubebuilder build tags, if possible.
const (
	CNIProviderCalico = "Calico"
	CNIProviderCilium = "Cilium"

	CSIProviderAWSEBS  = "aws-ebs"
	CSIProviderNutanix = "nutanix"

	VirtualIPProviderKubeVIP = "KubeVIP"

	ServiceLoadBalancerProviderMetalLB = "MetalLB"

	AddonStrategyClusterResourceSet AddonStrategy = "ClusterResourceSet"
	AddonStrategyHelmAddon          AddonStrategy = "HelmAddon"

	VolumeBindingImmediate            = storagev1.VolumeBindingImmediate
	VolumeBindingWaitForFirstConsumer = storagev1.VolumeBindingWaitForFirstConsumer

	VolumeReclaimRecycle = corev1.PersistentVolumeReclaimRecycle
	VolumeReclaimDelete  = corev1.PersistentVolumeReclaimDelete
	VolumeReclaimRetain  = corev1.PersistentVolumeReclaimRetain

	NutanixBootTypeLegacy = nutanixv1.NutanixBootTypeLegacy
	NutanixBootTypeUEFI   = nutanixv1.NutanixBootTypeUEFI
)

// FIXME: Remove StorageProvisioner from the API. Users do not provide this
// value; we derive it from the CSI provider.
type StorageProvisioner string

const (
	AWSEBSProvisioner  StorageProvisioner = "ebs.csi.aws.com"
	NutanixProvisioner StorageProvisioner = "csi.nutanix.com"
)

// FIXME: Remove the CCM providers from the API. Users do not provider this
// value; we derive it from the cluster infrastructure.
const (
	CCMProviderAWS     = "aws"
	CCMProviderNutanix = "nutanix"
)

type Addons struct {
	// +kubebuilder:validation:Optional
	CNI *CNI `json:"cni,omitempty"`

	// +kubebuilder:validation:Optional
	NFD *NFD `json:"nfd,omitempty"`

	// +kubebuilder:validation:Optional
	ClusterAutoscaler *ClusterAutoscaler `json:"clusterAutoscaler,omitempty"`

	// +kubebuilder:validation:Optional
	CCM *CCM `json:"ccm,omitempty"`

	// +kubebuilder:validation:Optional
	CSIProviders *CSI `json:"csi,omitempty"`

	// +optional
	ServiceLoadBalancer *ServiceLoadBalancer `json:"serviceLoadBalancer,omitempty"`
}

type AddonStrategy string

// CNI required for providing CNI configuration.
type CNI struct {
	// CNI provider to deploy.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Calico;Cilium
	Provider string `json:"provider"`
	// Addon strategy used to deploy the CNI provider to the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

// NFD tells us to enable or disable the node feature discovery addon.
type NFD struct {
	// Addon strategy used to deploy Node Feature Discovery (NFD) to the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

// ClusterAutoscaler tells us to enable or disable the cluster-autoscaler addon.
type ClusterAutoscaler struct {
	// Addon strategy used to deploy cluster-autoscaler to the management cluster
	// targeting the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

type DefaultStorage struct {
	// Name of the CSI Provider for the default storage class.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=aws-ebs;nutanix
	ProviderName string `json:"providerName"`

	// Name of storage class config in any of the provider objects.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	StorageClassConfigName string `json:"storageClassConfigName"`
}

type CSI struct {
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Providers []CSIProvider `json:"providers"`

	// +kubebuilder:validation:Required
	DefaultStorage DefaultStorage `json:"defaultStorage"`
}

type CSIProvider struct {
	// Name of the CSI Provider.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=aws-ebs;nutanix
	Name string `json:"name"`

	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	StorageClassConfig []StorageClassConfig `json:"storageClassConfig"`

	// Addon strategy used to deploy the CSI provider to the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`

	// The reference to any secret used by the CSI Provider.
	// +kubebuilder:validation:Optional
	Credentials *LocalObjectReference `json:"credentials,omitempty"`
}

type StorageClassConfig struct {
	// Name of storage class config.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Parameters passed into the storage class object.
	// +kubebuilder:validation:Optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=Delete;Retain;Recycle
	// +kubebuilder:default=Delete
	ReclaimPolicy *corev1.PersistentVolumeReclaimPolicy `json:"reclaimPolicy,omitempty"`

	// +kubebuilder:validation:Enum=Immediate;WaitForFirstConsumer
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=WaitForFirstConsumer
	VolumeBindingMode *storagev1.VolumeBindingMode `json:"volumeBindingMode,omitempty"`

	// If the storage class should allow volume expanding
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	AllowExpansion bool `json:"allowExpansion,omitempty"`
}

// CCM tells us to enable or disable the cloud provider interface.
type CCM struct {
	// A reference to the Secret for credential information for the target Prism Central instance
	// +kubebuilder:validation:Optional
	Credentials *LocalObjectReference `json:"credentials,omitempty"`
}

type ServiceLoadBalancer struct {
	// The LoadBalancer-type Service provider to deploy. Not required in infrastructures where
	// the CCM acts as the provider.
	// +kubebuilder:validation:Enum=MetalLB
	Provider string `json:"provider"`
}
