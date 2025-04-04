// Copyright 2024 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	vspherev1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-vsphere/apis/v1beta1"
)

type VSphereNodeSpec struct {
	MachineDetails VSphereMachineDetails `json:"machineDetails,omitempty"`
}

type VSphereMachineDetails struct {
	vspherev1.VirtualMachineCloneSpec `json:",inline"`
}
