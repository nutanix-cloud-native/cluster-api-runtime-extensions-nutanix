//go:build !ignore_autogenerated

/*
Copyright 2022 Nutanix

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	"github.com/nutanix-cloud-native/prism-go-client/environment/credentials"
	"k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	apiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/errors"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixCategoryIdentifier) DeepCopyInto(out *NutanixCategoryIdentifier) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixCategoryIdentifier.
func (in *NutanixCategoryIdentifier) DeepCopy() *NutanixCategoryIdentifier {
	if in == nil {
		return nil
	}
	out := new(NutanixCategoryIdentifier)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixCluster) DeepCopyInto(out *NutanixCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixCluster.
func (in *NutanixCluster) DeepCopy() *NutanixCluster {
	if in == nil {
		return nil
	}
	out := new(NutanixCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterList) DeepCopyInto(out *NutanixClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NutanixCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterList.
func (in *NutanixClusterList) DeepCopy() *NutanixClusterList {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterSpec) DeepCopyInto(out *NutanixClusterSpec) {
	*out = *in
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	if in.PrismCentral != nil {
		in, out := &in.PrismCentral, &out.PrismCentral
		*out = new(credentials.NutanixPrismEndpoint)
		(*in).DeepCopyInto(*out)
	}
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make([]NutanixFailureDomain, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterSpec.
func (in *NutanixClusterSpec) DeepCopy() *NutanixClusterSpec {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterStatus) DeepCopyInto(out *NutanixClusterStatus) {
	*out = *in
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make(apiv1beta1.FailureDomains, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(apiv1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.ClusterStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterStatus.
func (in *NutanixClusterStatus) DeepCopy() *NutanixClusterStatus {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterTemplate) DeepCopyInto(out *NutanixClusterTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterTemplate.
func (in *NutanixClusterTemplate) DeepCopy() *NutanixClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixClusterTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterTemplateList) DeepCopyInto(out *NutanixClusterTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NutanixClusterTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterTemplateList.
func (in *NutanixClusterTemplateList) DeepCopy() *NutanixClusterTemplateList {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixClusterTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterTemplateResource) DeepCopyInto(out *NutanixClusterTemplateResource) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterTemplateResource.
func (in *NutanixClusterTemplateResource) DeepCopy() *NutanixClusterTemplateResource {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixClusterTemplateSpec) DeepCopyInto(out *NutanixClusterTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixClusterTemplateSpec.
func (in *NutanixClusterTemplateSpec) DeepCopy() *NutanixClusterTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(NutanixClusterTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixFailureDomain) DeepCopyInto(out *NutanixFailureDomain) {
	*out = *in
	in.Cluster.DeepCopyInto(&out.Cluster)
	if in.Subnets != nil {
		in, out := &in.Subnets, &out.Subnets
		*out = make([]NutanixResourceIdentifier, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixFailureDomain.
func (in *NutanixFailureDomain) DeepCopy() *NutanixFailureDomain {
	if in == nil {
		return nil
	}
	out := new(NutanixFailureDomain)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixGPU) DeepCopyInto(out *NutanixGPU) {
	*out = *in
	if in.DeviceID != nil {
		in, out := &in.DeviceID, &out.DeviceID
		*out = new(int64)
		**out = **in
	}
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixGPU.
func (in *NutanixGPU) DeepCopy() *NutanixGPU {
	if in == nil {
		return nil
	}
	out := new(NutanixGPU)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixImageLookup) DeepCopyInto(out *NutanixImageLookup) {
	*out = *in
	if in.Format != nil {
		in, out := &in.Format, &out.Format
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixImageLookup.
func (in *NutanixImageLookup) DeepCopy() *NutanixImageLookup {
	if in == nil {
		return nil
	}
	out := new(NutanixImageLookup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachine) DeepCopyInto(out *NutanixMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachine.
func (in *NutanixMachine) DeepCopy() *NutanixMachine {
	if in == nil {
		return nil
	}
	out := new(NutanixMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineList) DeepCopyInto(out *NutanixMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NutanixMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineList.
func (in *NutanixMachineList) DeepCopy() *NutanixMachineList {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineSpec) DeepCopyInto(out *NutanixMachineSpec) {
	*out = *in
	out.MemorySize = in.MemorySize.DeepCopy()
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(NutanixResourceIdentifier)
		(*in).DeepCopyInto(*out)
	}
	if in.ImageLookup != nil {
		in, out := &in.ImageLookup, &out.ImageLookup
		*out = new(NutanixImageLookup)
		(*in).DeepCopyInto(*out)
	}
	in.Cluster.DeepCopyInto(&out.Cluster)
	if in.Subnets != nil {
		in, out := &in.Subnets, &out.Subnets
		*out = make([]NutanixResourceIdentifier, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AdditionalCategories != nil {
		in, out := &in.AdditionalCategories, &out.AdditionalCategories
		*out = make([]NutanixCategoryIdentifier, len(*in))
		copy(*out, *in)
	}
	if in.Project != nil {
		in, out := &in.Project, &out.Project
		*out = new(NutanixResourceIdentifier)
		(*in).DeepCopyInto(*out)
	}
	out.SystemDiskSize = in.SystemDiskSize.DeepCopy()
	if in.DataDisks != nil {
		in, out := &in.DataDisks, &out.DataDisks
		*out = make([]NutanixMachineVMDisk, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.BootstrapRef != nil {
		in, out := &in.BootstrapRef, &out.BootstrapRef
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.GPUs != nil {
		in, out := &in.GPUs, &out.GPUs
		*out = make([]NutanixGPU, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineSpec.
func (in *NutanixMachineSpec) DeepCopy() *NutanixMachineSpec {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineStatus) DeepCopyInto(out *NutanixMachineStatus) {
	*out = *in
	if in.Addresses != nil {
		in, out := &in.Addresses, &out.Addresses
		*out = make([]apiv1beta1.MachineAddress, len(*in))
		copy(*out, *in)
	}
	if in.NodeRef != nil {
		in, out := &in.NodeRef, &out.NodeRef
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(apiv1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.MachineStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineStatus.
func (in *NutanixMachineStatus) DeepCopy() *NutanixMachineStatus {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineTemplate) DeepCopyInto(out *NutanixMachineTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineTemplate.
func (in *NutanixMachineTemplate) DeepCopy() *NutanixMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixMachineTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineTemplateList) DeepCopyInto(out *NutanixMachineTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NutanixMachineTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineTemplateList.
func (in *NutanixMachineTemplateList) DeepCopy() *NutanixMachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NutanixMachineTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineTemplateResource) DeepCopyInto(out *NutanixMachineTemplateResource) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineTemplateResource.
func (in *NutanixMachineTemplateResource) DeepCopy() *NutanixMachineTemplateResource {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineTemplateSpec) DeepCopyInto(out *NutanixMachineTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineTemplateSpec.
func (in *NutanixMachineTemplateSpec) DeepCopy() *NutanixMachineTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineVMDisk) DeepCopyInto(out *NutanixMachineVMDisk) {
	*out = *in
	out.DiskSize = in.DiskSize.DeepCopy()
	if in.DeviceProperties != nil {
		in, out := &in.DeviceProperties, &out.DeviceProperties
		*out = new(NutanixMachineVMDiskDeviceProperties)
		**out = **in
	}
	if in.StorageConfig != nil {
		in, out := &in.StorageConfig, &out.StorageConfig
		*out = new(NutanixMachineVMStorageConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.DataSource != nil {
		in, out := &in.DataSource, &out.DataSource
		*out = new(NutanixResourceIdentifier)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineVMDisk.
func (in *NutanixMachineVMDisk) DeepCopy() *NutanixMachineVMDisk {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineVMDisk)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineVMDiskDeviceProperties) DeepCopyInto(out *NutanixMachineVMDiskDeviceProperties) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineVMDiskDeviceProperties.
func (in *NutanixMachineVMDiskDeviceProperties) DeepCopy() *NutanixMachineVMDiskDeviceProperties {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineVMDiskDeviceProperties)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixMachineVMStorageConfig) DeepCopyInto(out *NutanixMachineVMStorageConfig) {
	*out = *in
	if in.StorageContainer != nil {
		in, out := &in.StorageContainer, &out.StorageContainer
		*out = new(NutanixResourceIdentifier)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixMachineVMStorageConfig.
func (in *NutanixMachineVMStorageConfig) DeepCopy() *NutanixMachineVMStorageConfig {
	if in == nil {
		return nil
	}
	out := new(NutanixMachineVMStorageConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NutanixResourceIdentifier) DeepCopyInto(out *NutanixResourceIdentifier) {
	*out = *in
	if in.UUID != nil {
		in, out := &in.UUID, &out.UUID
		*out = new(string)
		**out = **in
	}
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NutanixResourceIdentifier.
func (in *NutanixResourceIdentifier) DeepCopy() *NutanixResourceIdentifier {
	if in == nil {
		return nil
	}
	out := new(NutanixResourceIdentifier)
	in.DeepCopyInto(out)
	return out
}
