// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/variables"
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

	// image identifies the image uploaded to Prism Central (PC). The identifier
	// (uuid or name) can be obtained from the console or API.
	Image NutanixResourceIdentifier `json:"image"`

	// cluster identifies the Prism Element in which the machine will be created.
	// The identifier (uuid or name) can be obtained from the console or API.
	Cluster NutanixResourceIdentifier `json:"cluster"`

	// subnet identifies the network subnet to use for the machine.
	// The identifier (uuid or name) can be obtained from the console or API.
	Subnets []NutanixResourceIdentifier `json:"subnets"`

	// List of categories that need to be added to the machines. Categories must already
	// exist in Prism Central. One category key can have more than one value.
	// +optional
	AdditionalCategories []NutanixCategoryIdentifier `json:"additionalCategories,omitempty"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	BootType NutanixBootType `json:"bootType,omitempty"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`

	// add the virtual machines to the project defined in Prism Central.
	// The project must already be present in the Prism Central.
	// +optional
	Project *NutanixResourceIdentifier `json:"project,omitempty"`
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
				"image": NutanixResourceIdentifier{}.VariableSchemaFromDescription(
					//nolint:lll // Long description.
					"image identifies the image uploaded to Prism Central (PC). The identifier (uuid or name) can be obtained from the console or API.",
				).OpenAPIV3Schema,
				"cluster": NutanixResourceIdentifier{}.VariableSchemaFromDescription(
					//nolint:lll // Long description.
					"cluster identifies the Prism Element in which the machine will be created. The identifier (uuid or name) can be obtained from the console or API.",
				).OpenAPIV3Schema,
				"subnets": {
					Type:        "array",
					Description: "subnets is a list of network subnets to use for the machine",
					Items: ptr.To(NutanixResourceIdentifier{}.VariableSchemaFromDescription(
						//nolint:lll // Long description.
						"subnet identifies the network subnet to use for the machine. The identifier (uuid or name) can be obtained from the console or API.",
					).OpenAPIV3Schema),
				},
				"additionalCategories": {
					Type: "array",
					//nolint:lll // Description is long.
					Description: "List of categories that need to be added to the machines. Categories must already exist in Prism Central. One category key can have more than one value.",
					Items: ptr.To(
						NutanixCategoryIdentifier{}.VariableSchema().OpenAPIV3Schema,
					),
				},
				"bootType": NutanixBootType(
					capxv1.NutanixBootTypeLegacy,
				).VariableSchema().
					OpenAPIV3Schema,
				"systemDiskSize": {
					Description: "systemDiskSize is size (in Quantity format) of the system disk of the VM eg. 20Gi",
					Type:        "string",
				},
				"project": NutanixResourceIdentifier{}.VariableSchemaFromDescription(
					//nolint:lll // Long description.
					"add the virtual machines to the project defined in Prism Central. The project must already be present in the Prism Central.",
				).OpenAPIV3Schema,
			},
			Required: []string{
				"vcpusPerSocket",
				"vcpuSockets",
				"memorySize",
				"image",
				"cluster",
				"subnets",
				"systemDiskSize",
			},
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

func (NutanixResourceIdentifier) VariableSchemaFromDescription(
	description string,
) clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Resource Identifier",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"type": NutanixIdentifierType(
					capxv1.NutanixIdentifierName,
				).VariableSchema().
					OpenAPIV3Schema,
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

type NutanixCategoryIdentifier capxv1.NutanixCategoryIdentifier

func (NutanixCategoryIdentifier) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Nutanix Category Identifier",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"key": {
					Type:        "string",
					Description: "key is the Key of category in PC.",
				},
				"value": {
					Type:        "string",
					Description: "value is the category value linked to the category key in PC.",
				},
			},
		},
	}
}
