// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
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

const (
	AddonStrategyClusterResourceSet AddonStrategy = "ClusterResourceSet"
	AddonStrategyHelmAddon          AddonStrategy = "HelmAddon"
)

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

type CSIProviders struct {
	// +optional
	Providers []CSIProvider `json:"providers,omitempty"`

	// +optional
	DefaultStorageClassName string `json:"defaultStorageClassName,omitempty"`
}

type CSIProvider struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	StorageClassConfig []StorageClassConfig `json:"storageClassConfig,omitempty"`

	// +optional
	Strategy AddonStrategy `json:"strategy,omitempty"`

	// +optional
	Credentials *corev1.SecretReference `json:"credentials,omitempty"`
}

type StorageClassConfig struct {
	// +optional
	Name string `json:"name,omitempty"`

	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

func (StorageClassConfig) VariableSchema() clusterv1.VariableSchema {
	supportedCSIProviders := []string{CSIProviderAWSEBS, CSIProviderNutanix}
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"name": {
					Type: "string",
					Enum: variables.MustMarshalValuesToEnumJSON(supportedCSIProviders...),
				},
				"parameters": {
					Type:                   "object",
					XPreserveUnknownFields: true,
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
				"storageClassConfig": StorageClassConfig{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

func (CSIProviders) VariableSchema() clusterv1.VariableSchema {
	supportedCSIProviders := []string{CSIProviderAWSEBS, CSIProviderNutanix}
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
				"defaultStorageClassName": {
					Type:        "string",
					Description: "Storage Class that will be made default for the cluster.",
					Enum: variables.MustMarshalValuesToEnumJSON(
						supportedCSIProviders...),
				},
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
