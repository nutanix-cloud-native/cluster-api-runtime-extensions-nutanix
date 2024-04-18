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
	Subnets NutanixResourceIdentifiers `json:"subnets"`

	// Defines the boot type of the virtual machine. Only supports UEFI and Legacy
	BootType NutanixBootType `json:"bootType,omitempty"`

	// systemDiskSize is size (in Quantity format) of the system disk of the VM
	// The minimum systemDiskSize is 20Gi bytes
	SystemDiskSize resource.Quantity `json:"systemDiskSize"`
}

// NutanixIdentifierType is an enumeration of different resource identifier types.
type NutanixIdentifierType capxv1.NutanixIdentifierType

// NutanixBootType is an enumeration of different boot types.
type NutanixBootType capxv1.NutanixBootType

type NutanixResourceIdentifier capxv1.NutanixResourceIdentifier

type NutanixResourceIdentifiers []NutanixResourceIdentifier
