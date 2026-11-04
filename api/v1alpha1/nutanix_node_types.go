// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/resource"

	capxv1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/github.com/nutanix-cloud-native/cluster-api-provider-nutanix/api/v1beta1"
)

type NutanixControlPlaneNodeSpec struct {
	// +kubebuilder:validation:Required
	MachineDetails NutanixMachineDetails `json:"machineDetails"`

	// failureDomains specifies a list of NutanixFailureDomains (by names)
	// that the cluster uses to deploy its control-plane machines.
	// +listType=set
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=5
	// +kubebuilder:validation:items:MaxLength=253
	FailureDomains []string `json:"failureDomains,omitempty"`
}

type NutanixWorkerNodeSpec struct {
	// +kubebuilder:validation:Required
	MachineDetails NutanixMachineDetails `json:"machineDetails"`
}

// +kubebuilder:validation:XValidation:rule="has(self.image) != has(self.imageLookup)",message="Either 'image' or 'imageLookup' must be set, but not both"
// +kubebuilder:validation:XValidation:rule="!has(self.vmProfile) || (!has(self.vcpuSockets) && !has(self.vcpusPerSocket) && !has(self.memorySize) && !has(self.bootType) && (!has(self.gpus) || size(self.gpus) == 0))",message="When 'vmProfile' is set, 'vcpuSockets', 'vcpusPerSocket', 'memorySize', 'bootType', and 'gpus' must not be set"
// +kubebuilder:validation:XValidation:rule="has(self.vmProfile) || (has(self.vcpuSockets) && has(self.vcpusPerSocket) && has(self.memorySize) && has(self.bootType))",message="When 'vmProfile' is not set, 'vcpuSockets', 'vcpusPerSocket', 'memorySize', and 'bootType' must be set"
type NutanixMachineDetails struct {
	// vcpusPerSocket is the number of vCPUs per socket of the VM
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +optional
	VCPUsPerSocket int32 `json:"vcpusPerSocket,omitzero"`

	// vcpuSockets is the number of vCPU sockets of the VM
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +optional
	VCPUSockets int32 `json:"vcpuSockets,omitzero"`

	// memorySize is the memory size (in Quantity format) of the VM
	// +kubebuilder:validation:Optional
	// +optional
	MemorySize resource.Quantity `json:"memorySize,omitzero"`

	// image identifies the image uploaded to Prism Central (PC). The identifier
	// (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Optional
	Image *capxv1.NutanixResourceIdentifier `json:"image,omitempty"`

	// imageLookup is a container that holds how to look up vm images for the cluster.
	// +kubebuilder:validation:Optional
	ImageLookup *capxv1.NutanixImageLookup `json:"imageLookup,omitempty"`

	// cluster identifies the Prism Element in which the machine will be created.
	// The identifier (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Optional
	Cluster *capxv1.NutanixResourceIdentifier `json:"cluster,omitempty"`

	// subnet identifies the network subnet to use for the machine.
	// The identifier (uuid or name) can be obtained from the console or API.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=16
	Subnets []capxv1.NutanixResourceIdentifier `json:"subnets,omitempty"`

	// List of categories that need to be added to the machines. Categories must already
	// exist in Prism Central. One category key can have more than one value.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	AdditionalCategories []capxv1.NutanixCategoryIdentifier `json:"additionalCategories,omitempty"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum:=legacy;uefi
	// +optional
	BootType capxv1.NutanixBootType `json:"bootType,omitempty"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	// +kubebuilder:validation:Required
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`

	// add the virtual machines to the project defined in Prism Central.
	// The project must already be present in the Prism Central.
	// +kubebuilder:validation:Optional
	Project *capxv1.NutanixResourceIdentifier `json:"project,omitempty"`

	// List of GPU devices that need to be added to the machines.
	// Mutually exclusive with VMProfile. When VMProfile is set, GPUs must not be set.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	// +optional
	GPUs []capxv1.NutanixGPU `json:"gpus,omitempty"`

	// VMProfile is to identify the VM profile in Prism Central (PC)
	// The VM profile identifier (uuid or name) can be obtained from the Prism Central console
	// or using the prism_central API.
	// When VMProfile is set, it provides CPU, memory, and bootType configuration,
	// making vcpusPerSocket, vcpuSockets, memorySize, bootType, and GPUs mutually exclusive.
	// +kubebuilder:validation:Optional
	// +optional
	VMProfile *capxv1.NutanixResourceIdentifier `json:"vmProfile,omitempty"`
}

// HasClusterAndSubnets checks if cluster and subnets are configured in the machine details.
// It returns (hasCluster, hasSubnets).
func (md *NutanixMachineDetails) HasClusterAndSubnets() (hasCluster, hasSubnets bool) {
	hasCluster = md.Cluster != nil &&
		(md.Cluster.IsName() || md.Cluster.IsUUID())
	hasSubnets = len(md.Subnets) > 0
	return hasCluster, hasSubnets
}
