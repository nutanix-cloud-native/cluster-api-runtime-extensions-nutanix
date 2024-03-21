// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
)

const (
	AddonStrategyClusterResourceSet AddonStrategy = "ClusterResourceSet"
	AddonStrategyHelmAddon          AddonStrategy = "HelmAddon"

	VolumeBindingImmediate            string = "Immmediate"
	VolumeBindingWaitForFirstConsumer string = "WaitForFirstConsumer"

	VolumeReclaimRecycle string = "Recycle"
	VolumeReclaimDelete  string = "Delete"
	VolumeReclaimRetain  string = "Retain"
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
	CSIProviders *CSIProviders `json:"csi,omitempty"`
}

func (Addons) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"cni":               CNI{}.VariableSchema().OpenAPIV3Schema,
				"nfd":               NFD{}.VariableSchema().OpenAPIV3Schema,
				"clusterAutoscaler": ClusterAutoscaler{}.VariableSchema().OpenAPIV3Schema,
				"csi":               CSIProviders{}.VariableSchema().OpenAPIV3Schema,
				"ccm":               CCM{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

type AddonStrategy string

// CNI required for providing CNI configuration.
type CNI struct {
	// +optional
	Provider string `json:"provider,omitempty"`
	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`
}

func (CNI) VariableSchema() clusterv1.VariableSchema {
	supportedCNIProviders := []string{CNIProviderCalico, CNIProviderCilium}

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"provider": {
					Description: "CNI provider to deploy",
					Type:        "string",
					Enum:        variables.MustMarshalValuesToEnumJSON(supportedCNIProviders...),
				},
				"strategy": {
					Description: "Addon strategy used to deploy the CNI provider to the workload cluster",
					Type:        "string",
					Enum: variables.MustMarshalValuesToEnumJSON(
						AddonStrategyClusterResourceSet,
						AddonStrategyHelmAddon,
					),
				},
			},
			Required: []string{"provider", "strategy"},
		},
	}
}

// NFD tells us to enable or disable the node feature discovery addon.
type NFD struct {
	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`
}

func (NFD) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"strategy": {
					Description: "Addon strategy used to deploy Node Feature Discovery (NFD) to the workload cluster",
					Type:        "string",
					Enum: variables.MustMarshalValuesToEnumJSON(
						AddonStrategyClusterResourceSet,
						AddonStrategyHelmAddon,
					),
				},
			},
			Required: []string{"strategy"},
		},
	}
}

// ClusterAutoscaler tells us to enable or disable the cluster-autoscaler addon.
type ClusterAutoscaler struct {
	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`
}

func (ClusterAutoscaler) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"strategy": {
					Description: "Addon strategy used to deploy cluster-autoscaler to the management cluster," +
						"targeting the workload cluster.",
					Type: "string",
					Enum: variables.MustMarshalValuesToEnumJSON(
						AddonStrategyClusterResourceSet,
						AddonStrategyHelmAddon,
					),
				},
			},
			Required: []string{"strategy"},
		},
	}
}

type DefaultStorage struct {
	ProviderName           string `json:"providerName"`
	StorageClassConfigName string `json:"storageClassConfigName"`
}

type CSIProviders struct {
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
	Credentials *corev1.SecretReference `json:"credentials,omitempty"`
}

type StorageClassConfig struct {
	Name string `json:"name"`

	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// +optional
	ReclaimPolicy string `json:"reclaimPolicy,omitempty"`

	// +optional
	VolumeBindingMode string `json:"volumeBindingMode,omitempty"`
}

func (StorageClassConfig) VariableSchema() clusterv1.VariableSchema {
	supportedReclaimPolicies := []string{
		VolumeReclaimRecycle,
		VolumeReclaimDelete,
		VolumeReclaimRetain,
	}
	supportedBindingModes := []string{
		VolumeBindingImmediate,
		VolumeBindingWaitForFirstConsumer,
	}
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"name": {
					Type:        "string",
					Description: "Name of storage class config.",
				},
				"parameters": {
					Type:                   "object",
					Description:            "Parameters passed into the storage class object.",
					XPreserveUnknownFields: true,
				},
				"reclaimPolicy": {
					Type: "string",
					Enum: variables.MustMarshalValuesToEnumJSON(supportedReclaimPolicies...),
				},
				"volumeBindingMode": {
					Type: "string",
					Enum: variables.MustMarshalValuesToEnumJSON(supportedBindingModes...),
				},
			},
		},
	}
}

func (CSIProvider) VariableSchema() clusterv1.VariableSchema {
	supportedCSIProviders := []string{CSIProviderAWSEBS, CSIProviderNutanix}
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"name": {
					Description: "Name of the CSI Provider",
					Type:        "string",
					Enum: variables.MustMarshalValuesToEnumJSON(
						supportedCSIProviders...),
				},
				"strategy": {
					Description: "Addon strategy used to deploy the CSI provider to the workload cluster",
					Type:        "string",
					Enum: variables.MustMarshalValuesToEnumJSON(
						AddonStrategyClusterResourceSet,
						AddonStrategyHelmAddon,
					),
				},
				"credentials": {
					Type:        "object",
					Description: "The reference to any secret used by the CSI Provider.",
					Properties: map[string]clusterv1.JSONSchemaProps{
						"name": {
							Type: "string",
						},
						"namespace": {
							Type: "string",
						},
					},
				},
				"storageClassConfig": {
					Type: "array",
					Items: &clusterv1.JSONSchemaProps{
						Type:       "object",
						Properties: StorageClassConfig{}.VariableSchema().OpenAPIV3Schema.Properties,
					},
				},
			},
		},
	}
}

func (DefaultStorage) VariableSchema() clusterv1.VariableSchema {
	supportedCSIProviders := []string{CSIProviderAWSEBS, CSIProviderNutanix}
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "object",
			Description: "A tuple of provider name and storage class ",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"providerName": {
					Type:        "string",
					Description: "Name of the CSI Provider for the default storage class",
					Enum: variables.MustMarshalValuesToEnumJSON(
						supportedCSIProviders...),
				},
				"storageClassConfigName": {
					Type:        "string",
					Description: "Name of storage class config in any of the provider objects",
				},
			},
		},
	}
}

func (CSIProviders) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"providers": {
					Type: "array",
					Items: &clusterv1.JSONSchemaProps{
						Type:       "object",
						Properties: CSIProvider{}.VariableSchema().OpenAPIV3Schema.Properties,
					},
				},
				"defaultStorage": DefaultStorage{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

// CCM tells us to enable or disable the cloud provider interface.
type CCM struct{}

func (CCM) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
		},
	}
}
