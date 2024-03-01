// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	capxv1 "github.com/d2iq-labs/capi-runtime-extensions/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/d2iq-labs/capi-runtime-extensions/api/variables"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// NutanixIdentifierType is an enumeration of different resource identifier types.
type NutanixIdentifierType string

// NutanixBootType is an enumeration of different boot types.
type NutanixBootType string

// NutanixGPUIdentifierType is an enumeration of different resource identifier types for GPU entities.
type NutanixGPUIdentifierType string

type NutanixNodeSpec struct {
	// vcpusPerSocket is the number of vCPUs per socket of the VM
	VCPUsPerSocket int32 `json:"vcpusPerSocket"`

	// vcpuSockets is the number of vCPU sockets of the VM
	VCPUSockets int32 `json:"vcpuSockets"`

	// memorySize is the memory size (in Quantity format) of the VM
	MemorySize string `json:"memorySize"`

	// image is to identify the rhcos image uploaded to the Prism Central (PC)
	// The image identifier (uuid or name) can be obtained from the Prism Central console
	// or using the prism_central API.
	Image NutanixResourceIdentifier `json:"image"`

	// cluster is to identify the cluster (the Prism Element under management
	// of the Prism Central), in which the Machine's VM will be created.
	// The cluster identifier (uuid or name) can be obtained from the Prism Central console
	// or using the prism_central API.
	Cluster NutanixResourceIdentifier `json:"cluster"`

	// subnet is to identify the cluster's network subnet to use for the Machine's VM
	// The cluster identifier (uuid or name) can be obtained from the Prism Central console
	// or using the prism_central API.
	Subnets NutanixResourceIdentifiers `json:"subnet"`

	// List of categories that need to be added to the machines. Categories must already exist in Prism Central
	AdditionalCategories []NutanixCategoryIdentifier `json:"additionalCategories,omitempty"`

	// Add the machine resources to a Prism Central project
	Project *NutanixResourceIdentifier `json:"project,omitempty"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	BootType string `json:"bootType,omitempty"` //TODO use NutanixBootType enum somehow

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	SystemDiskSize string `json:"systemDiskSize"`

	// List of GPU devices that need to be added to the machines.
	GPUs []NutanixGPU `json:"gpus,omitempty"`
}

func (NutanixNodeSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Node configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"vcpusPerSocket": {
					Description: "vcpusPerSocket is the number of vCPUs per socket of the VM",
					Type:        "integer",
				},
				"vcpuSockets": {
					Description: "vcpuSockets is the number of vCPU sockets of the VM",
					Type:        "integer",
				},
				"memorySize": {
					Description: "memorySize is the memory size (in Quantity format) of the VM eg. 4Gi",
					Type:        "string",
				},
				"image":   NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema,
				"cluster": NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema,
				"subnet":  NutanixResourceIdentifiers{}.VariableSchema().OpenAPIV3Schema,
				"bootType": {
					Description: "Defines the boot type of the virtual machine. Only supports UEFI and Legacy",
					Type:        "string",
				},
				"systemDiskSize": {
					Description: "systemDiskSize is size (in Quantity format) of the system disk of the VM eg. 20Gi",
					Type:        "string",
				},
				// "project": {},
				// "additionalCategories": {},
				// "gpus": {},
			},
		},
	}
}

func (NutanixBootType) VariableSchema() clusterv1.VariableSchema {
	supportedBootType := []capxv1.NutanixBootType{
		capxv1.NutanixBootTypeLegacy,
		capxv1.NutanixBootTypeUEFI,
	}

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Boot type enum",
			Type:        "string",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"bootType": {
					Description: "Defines the boot type of the virtual machine. Only supports UEFI and Legacy",
					Type:        "string",
					Enum:        variables.MustMarshalValuesToEnumJSON(supportedBootType...),
				},
			},
		},
	}
}

type NutanixResourceIdentifier struct {
	// Type is the identifier type to use for this resource.
	Type NutanixIdentifierType `json:"type"`

	// uuid is the UUID of the resource in the PC.
	// +optional
	UUID *string `json:"uuid,omitempty"`

	// name is the resource name in the PC
	// +optional
	Name *string `json:"name,omitempty"`
}

func (NutanixResourceIdentifier) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Resource Identifier",
			Type:        "object",
			Properties:  map[string]clusterv1.JSONSchemaProps{},
		},
	}
}

type NutanixCategoryIdentifier struct {
	// key is the Key of category in PC.
	// +optional
	Key string `json:"key,omitempty"`

	// value is the category value linked to the category key in PC
	// +optional
	Value string `json:"value,omitempty"`
}

func (NutanixCategoryIdentifier) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Category Identifier",
			Type:        "object",
			Properties:  map[string]clusterv1.JSONSchemaProps{},
		},
	}
}

type NutanixGPU struct {
	// Type is the identifier type to use for this resource.
	Type NutanixGPUIdentifierType `json:"type"`

	// deviceID is the id of the GPU entity.
	// +optional
	DeviceID *int64 `json:"deviceID,omitempty"`

	// name is the GPU name
	// +optional
	Name *string `json:"name,omitempty"`
}

func (NutanixGPU) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix GPU type",
			Type:        "object",
			Properties:  map[string]clusterv1.JSONSchemaProps{},
		},
	}
}
