// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/variables"
)

type NutanixNodeSpec struct {
	MachineDetails NutanixMachineDetails `json:"machineDetails"`
}

func (NutanixNodeSpec) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Node configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"machineDetails": NutanixMachineDetails{}.VariableSchema().OpenAPIV3Schema,
			},
			Required: []string{"machineDetails"},
		},
	}
}

type NutanixMachineDetails struct {
	// vcpusPerSocket is the number of vCPUs per socket of the VM
	VCPUsPerSocket int32 `json:"vcpusPerSocket"`

	// vcpuSockets is the number of vCPU sockets of the VM
	VCPUSockets int32 `json:"vcpuSockets"`

	// memorySize is the memory size (in Quantity format) of the VM
	MemorySize resource.Quantity `json:"memorySize"`

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
	Subnets NutanixResourceIdentifiers `json:"subnets"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	BootType NutanixBootType `json:"bootType,omitempty"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`
}

func (NutanixMachineDetails) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Machine configuration",
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
				"image":    NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema,
				"cluster":  NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema,
				"subnets":  NutanixResourceIdentifiers{}.VariableSchema().OpenAPIV3Schema,
				"bootType": NutanixBootType(capxv1.NutanixBootTypeLegacy).VariableSchema().OpenAPIV3Schema,
				"systemDiskSize": {
					Description: "systemDiskSize is size (in Quantity format) of the system disk of the VM eg. 20Gi",
					Type:        "string",
				},
			},
			Required: []string{"vcpusPerSocket", "vcpuSockets", "memorySize", "image", "cluster", "subnets", "systemDiskSize"},
		},
	}
}

// NutanixIdentifierType is an enumeration of different resource identifier types.
type NutanixIdentifierType capxv1.NutanixIdentifierType

func (NutanixIdentifierType) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Description: "NutanixIdentifierType is an enumeration of different resource identifier types",
			Enum: variables.MustMarshalValuesToEnumJSON(
				capxv1.NutanixIdentifierUUID,
				capxv1.NutanixIdentifierName,
			),
		},
	}
}

// NutanixBootType is an enumeration of different boot types.
type NutanixBootType capxv1.NutanixBootType

func (NutanixBootType) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:        "string",
			Description: "NutanixBootType is an enumeration of different boot types.",
			Enum: variables.MustMarshalValuesToEnumJSON(
				capxv1.NutanixBootTypeLegacy,
				capxv1.NutanixBootTypeUEFI,
			),
		},
	}
}

type NutanixResourceIdentifier capxv1.NutanixResourceIdentifier

func (NutanixResourceIdentifier) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Resource Identifier",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"type": NutanixIdentifierType(capxv1.NutanixIdentifierName).VariableSchema().OpenAPIV3Schema,
				"uuid": {
					Type:        "string",
					Description: "uuid is the UUID of the resource in the PC.",
				},
				"name": {
					Type:        "string",
					Description: "name is the resource name in the PC.",
				},
			},
		},
	}
}

type NutanixResourceIdentifiers []NutanixResourceIdentifier

func (NutanixResourceIdentifiers) VariableSchema() clusterv1.VariableSchema {
	resourceSchema := NutanixResourceIdentifier{}.VariableSchema().OpenAPIV3Schema

	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix resource identifier",
			Type:        "array",
			Items:       &resourceSchema,
		},
	}
}
