// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
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
	DefaultClassName string `json:"defaultClassName,omitempty"`
}

type CSIProvider struct {
	Name string `json:"name,omitempty"`
}

func (CSIProviders) VariableSchema() clusterv1.VariableSchema {
	supportedCSIProviders := []string{CSIProviderAWSEBS}
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"providers": {
					Type: "array",
					Items: &clusterv1.JSONSchemaProps{
						Type: "object",
						Properties: map[string]clusterv1.JSONSchemaProps{
							"name": {
								Type: "string",
								Enum: variables.MustMarshalValuesToEnumJSON(
									supportedCSIProviders...),
							},
						},
					},
				},
				"defaultClassName": {
					Type: "string",
					Enum: variables.MustMarshalValuesToEnumJSON(supportedCSIProviders...),
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
