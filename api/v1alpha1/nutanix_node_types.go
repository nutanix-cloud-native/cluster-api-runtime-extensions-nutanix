// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

type NutanixNodeSpec struct {
	MachineDetails NutanixMachineDetails `json:"machineDetails"`
}

// +kubebuilder:validation:XValidation:rule="has(self.image) != has(self.imageLookup)",message="Either 'image' or 'imageLookup' must be set, but not both"
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
	// +kubebuilder:validation:Optional
	// +optional
	Image *capxv1.NutanixResourceIdentifier `json:"image,omitempty"`

	// imageLookup is a container that holds how to look up vm images for the cluster.
	// +kubebuilder:validation:Optional
	// +optional
	ImageLookup *capxv1.NutanixImageLookup `json:"imageLookup,omitempty"`

	// cluster identifies the Prism Element in which the machine will be created.
	// The identifier (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Required
	Cluster capxv1.NutanixResourceIdentifier `json:"cluster"`

	// subnet identifies the network subnet to use for the machine.
	// The identifier (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Required
	Subnets []capxv1.NutanixResourceIdentifier `json:"subnets"`

	// List of categories that need to be added to the machines. Categories must already
	// exist in Prism Central. One category key can have more than one value.
	// +kubebuilder:validation:Optional
	AdditionalCategories []capxv1.NutanixCategoryIdentifier `json:"additionalCategories,omitempty"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum:=legacy;uefi
	BootType capxv1.NutanixBootType `json:"bootType"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	// +kubebuilder:validation:Required
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`

	// add the virtual machines to the project defined in Prism Central.
	// The project must already be present in the Prism Central.
	// +kubebuilder:validation:Optional
	Project *capxv1.NutanixResourceIdentifier `json:"project,omitempty"`

	// List of GPU devices that need to be added to the machines.
	// +kubebuilder:validation:Optional
	GPUs []capxv1.NutanixGPU `json:"gpus,omitempty"`
}
