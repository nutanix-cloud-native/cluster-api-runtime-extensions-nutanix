// Copyright 2024 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

type NutanixNodeSpec struct {
	MachineDetails NutanixMachineDetails `json:"machineDetails"`
}

type NutanixMachineDetails struct {
	// vcpusPerSocket is the number of vCPUs per socket of the VM
	// +kubebuilder:validation:Required
	VCPUsPerSocket int32 `json:"vcpusPerSocket"`

	// vcpuSockets is the number of vCPU sockets of the VM
	// +kubebuilder:validation:Required
	VCPUSockets int32 `json:"vcpuSockets"`

	// memorySize is the memory size (in Quantity format) of the VM
	// +kubebuilder:validation:Required
	MemorySize resource.Quantity `json:"memorySize"`

	// image identifies the image uploaded to Prism Central (PC). The identifier
	// (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Required
	Image NutanixResourceIdentifier `json:"image"`

	// cluster identifies the Prism Element in which the machine will be created.
	// The identifier (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Required
	Cluster NutanixResourceIdentifier `json:"cluster"`

	// subnet identifies the network subnet to use for the machine.
	// The identifier (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Required
	Subnets []NutanixResourceIdentifier `json:"subnets"`

	// List of categories that need to be added to the machines. Categories must already
	// exist in Prism Central. One category key can have more than one value.
	// +kubebuilder:validation:Optional
	AdditionalCategories []NutanixCategoryIdentifier `json:"additionalCategories,omitempty"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum:=legacy;uefi
	BootType NutanixBootType `json:"bootType"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	// +kubebuilder:validation:Required
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`

	// add the virtual machines to the project defined in Prism Central.
	// The project must already be present in the Prism Central.
	// +kubebuilder:validation:Optional
	Project *NutanixResourceIdentifier `json:"project,omitempty"`

	// List of GPU devices that need to be added to the machines.
	// +kubebuilder:validation:Optional
	GPUs []NutanixGPU `json:"gpus,omitempty"`
}

type NutanixGPU struct {
	// Type is the identifier type to use for this resource.
	// +kubebuilder:validation:Required
	Type NutanixGPUIdentifierType `json:"type"`

	// DeviceID is the id of the GPU entity.
	// +kubebuilder:validation:Optional
	DeviceID *int64 `json:"deviceID,omitempty"`

	// Name is the GPU name
	// +kubebuilder:validation:Optional
	Name *string `json:"name,omitempty"`
}

// NutanixIdentifierType is an enumeration of different resource identifier types.
type NutanixIdentifierType capxv1.NutanixIdentifierType

// NutanixBootType is an enumeration of different boot types.
type NutanixBootType capxv1.NutanixBootType

type NutanixResourceIdentifier capxv1.NutanixResourceIdentifier

type NutanixCategoryIdentifier capxv1.NutanixCategoryIdentifier

type NutanixGPUIdentifierType capxv1.NutanixGPUIdentifierType
