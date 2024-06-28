// Copyright 2024 Nutanix. All rights reserved.
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

	CSIProviderAWSEBS    = "aws-ebs"
	CSIProviderNutanix   = "nutanix"
	CSIProviderLocalPath = "local-path"

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
	AWSEBSProvisioner    StorageProvisioner = "ebs.csi.aws.com"
	NutanixProvisioner   StorageProvisioner = "csi.nutanix.com"
	LocalPathProvisioner StorageProvisioner = "rancher.io/local-path"
)

// FIXME: Remove the CCM providers from the API. Users do not provider this
// value; we derive it from the cluster infrastructure.
const (
	CCMProviderAWS     = "aws"
	CCMProviderNutanix = "nutanix"
)

type AWSAddons struct {
	GenericAddons `json:",inline"`

	// +kubebuilder:validation:Optional
	CSI *AWSCSI `json:"csi,omitempty"`
}

type DockerAddons struct {
	GenericAddons `json:",inline"`

	// +kubebuilder:validation:Optional
	CSI *DockerCSI `json:"csi,omitempty"`
}

type NutanixAddons struct {
	GenericAddons `json:",inline"`

	// +kubebuilder:validation:Optional
	CSI *NutanixCSI `json:"csi,omitempty"`
}

type GenericAddons struct {
	// +kubebuilder:validation:Optional
	CNI *CNI `json:"cni,omitempty"`

	// +kubebuilder:validation:Optional
	NFD *NFD `json:"nfd,omitempty"`

	// +kubebuilder:validation:Optional
	ClusterAutoscaler *ClusterAutoscaler `json:"clusterAutoscaler,omitempty"`

	// +kubebuilder:validation:Optional
	CCM *CCM `json:"ccm,omitempty"`

	// +kubebuilder:validation:Optional
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

type GenericCSI struct {
	// +kubebuilder:validation:Required
	DefaultStorage DefaultStorage `json:"defaultStorage"`

	// Deploy the CSI snapshot controller and associated CRDs.
	// +kubebuilder:validation:Optional
	SnapshotController *SnapshotController `json:"snapshotController,omitempty"`
}

type SnapshotController struct {
	// Addon strategy used to deploy the snapshot controller to the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

type DefaultStorage struct {
	// Name of the CSI Provider for the default storage class.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=aws-ebs;nutanix;local-path
	Provider string `json:"provider"`

	// Name of the default storage class config the specified default provider.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	StorageClassConfig string `json:"storageClassConfig"`
}

type AWSCSI struct {
	GenericCSI `json:",inline"`

	// +kubebuilder:validation:Required
	Providers AWSCSIProviders `json:"providers"`
}

type AWSCSIProviders struct {
	// +kubebuilder:validation:Required
	AWSEBSCSI CSIProvider `json:"aws-ebs"`
}

type DockerCSI struct {
	GenericCSI `json:",inline"`

	// +kubebuilder:validation:Required
	Providers DockerCSIProviders `json:"providers"`
}

type DockerCSIProviders struct {
	// +kubebuilder:validation:Required
	LocalPathCSI CSIProvider `json:"local-path"`
}

type NutanixCSI struct {
	GenericCSI `json:",inline"`

	// +kubebuilder:validation:Required
	Providers NutanixCSIProviders `json:"providers"`
}

type NutanixCSIProviders struct {
	// +kubebuilder:validation:Required
	NutanixCSI CSIProvider `json:"nutanix"`
}

type CSIProvider struct {
	// StorageClassConfigs is a map of storage class configurations for this CSI provider.
	// +kubebuilder:validation:Required
	StorageClassConfigs map[string]StorageClassConfig `json:"storageClassConfigs"`

	// Addon strategy used to deploy the CSI provider to the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`

	// The reference to any secret used by the CSI Provider.
	// +kubebuilder:validation:Optional
	Credentials *CSICredentials `json:"credentials,omitempty"`
}

type StorageClassConfig struct {
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

type CSICredentials struct {
	// A reference to the Secret containing the credentials used by the CSI provider.
	// +kubebuilder:validation:Required
	SecretRef LocalObjectReference `json:"secretRef"`
}

// CCM tells us to enable or disable the cloud provider interface.
type CCM struct {
	// A reference to the Secret for credential information for the target Prism Central instance
	// +kubebuilder:validation:Optional
	Credentials *CCMCredentials `json:"credentials,omitempty"`

	// Addon strategy used to deploy the CCM to the workload cluster.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ClusterResourceSet;HelmAddon
	Strategy AddonStrategy `json:"strategy"`
}

type CCMCredentials struct {
	// A reference to the Secret containing the credentials used by the CCM provider.
	// +kubebuilder:validation:Required
	SecretRef LocalObjectReference `json:"secretRef"`
}

type ServiceLoadBalancer struct {
	// The LoadBalancer-type Service provider to deploy. Not required in infrastructures where
	// the CCM acts as the provider.
	// +kubebuilder:validation:Enum=MetalLB
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`

	// Configuration for the chosen ServiceLoadBalancer provider.
	// +kubebuilder:validation:Optional
	Configuration *ServiceLoadBalancerConfiguration `json:"configuration"`
}

type ServiceLoadBalancerConfiguration struct {
	// AddressRanges is a list of IPv4 or IPv6 address ranges from which
	// the provider can choose a load balancer address.
	AddressRanges []AddressRange `json:"addressRanges"`
}

// AddressRange defines an IPv4 or IPv6 range.
type AddressRange struct {
	// FIXME Both start and end should be of the same type (IPv4 or IPv6).
	// Because the pattern accepts either type, start and end can be of different
	// types.
	// Source: https://github.com/metal3-io/ip-address-manager/blob/b4f7c2a/api/v1alpha1/common_types.go#L19

	// +kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$))"
	Start string `json:"start"`

	// +kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$))"
	End string `json:"end"`
}
