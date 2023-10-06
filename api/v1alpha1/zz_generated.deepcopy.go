//go:build !ignore_autogenerated

// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AWSSpec) DeepCopyInto(out *AWSSpec) {
	*out = *in
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(Region)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AWSSpec.
func (in *AWSSpec) DeepCopy() *AWSSpec {
	if in == nil {
		return nil
	}
	out := new(AWSSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Addons) DeepCopyInto(out *Addons) {
	*out = *in
	if in.CNI != nil {
		in, out := &in.CNI, &out.CNI
		*out = new(CNI)
		**out = **in
	}
	if in.NFD != nil {
		in, out := &in.NFD, &out.NFD
		*out = new(NFD)
		**out = **in
	}
	if in.CSIProviders != nil {
		in, out := &in.CSIProviders, &out.CSIProviders
		*out = new(CSIProviders)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Addons.
func (in *Addons) DeepCopy() *Addons {
	if in == nil {
		return nil
	}
	out := new(Addons)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CNI) DeepCopyInto(out *CNI) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CNI.
func (in *CNI) DeepCopy() *CNI {
	if in == nil {
		return nil
	}
	out := new(CNI)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CSIProvider) DeepCopyInto(out *CSIProvider) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CSIProvider.
func (in *CSIProvider) DeepCopy() *CSIProvider {
	if in == nil {
		return nil
	}
	out := new(CSIProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CSIProviders) DeepCopyInto(out *CSIProviders) {
	*out = *in
	if in.Providers != nil {
		in, out := &in.Providers, &out.Providers
		*out = make([]CSIProvider, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CSIProviders.
func (in *CSIProviders) DeepCopy() *CSIProviders {
	if in == nil {
		return nil
	}
	out := new(CSIProviders)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterConfig) DeepCopyInto(out *ClusterConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterConfig.
func (in *ClusterConfig) DeepCopy() *ClusterConfig {
	if in == nil {
		return nil
	}
	out := new(ClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterConfigSpec) DeepCopyInto(out *ClusterConfigSpec) {
	*out = *in
	if in.AWS != nil {
		in, out := &in.AWS, &out.AWS
		*out = new(AWSSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Docker != nil {
		in, out := &in.Docker, &out.Docker
		*out = new(DockerSpec)
		(*in).DeepCopyInto(*out)
	}
	in.GenericClusterConfig.DeepCopyInto(&out.GenericClusterConfig)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterConfigSpec.
func (in *ClusterConfigSpec) DeepCopy() *ClusterConfigSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DockerSpec) DeepCopyInto(out *DockerSpec) {
	*out = *in
	if in.CustomImage != nil {
		in, out := &in.CustomImage, &out.CustomImage
		*out = new(OCIImage)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DockerSpec.
func (in *DockerSpec) DeepCopy() *DockerSpec {
	if in == nil {
		return nil
	}
	out := new(DockerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Etcd) DeepCopyInto(out *Etcd) {
	*out = *in
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(Image)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Etcd.
func (in *Etcd) DeepCopy() *Etcd {
	if in == nil {
		return nil
	}
	out := new(Etcd)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in ExtraAPIServerCertSANs) DeepCopyInto(out *ExtraAPIServerCertSANs) {
	{
		in := &in
		*out = make(ExtraAPIServerCertSANs, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExtraAPIServerCertSANs.
func (in ExtraAPIServerCertSANs) DeepCopy() ExtraAPIServerCertSANs {
	if in == nil {
		return nil
	}
	out := new(ExtraAPIServerCertSANs)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenericClusterConfig) DeepCopyInto(out *GenericClusterConfig) {
	*out = *in
	if in.KubernetesImageRepository != nil {
		in, out := &in.KubernetesImageRepository, &out.KubernetesImageRepository
		*out = new(KubernetesImageRepository)
		**out = **in
	}
	if in.Etcd != nil {
		in, out := &in.Etcd, &out.Etcd
		*out = new(Etcd)
		(*in).DeepCopyInto(*out)
	}
	if in.Proxy != nil {
		in, out := &in.Proxy, &out.Proxy
		*out = new(HTTPProxy)
		(*in).DeepCopyInto(*out)
	}
	if in.ExtraAPIServerCertSANs != nil {
		in, out := &in.ExtraAPIServerCertSANs, &out.ExtraAPIServerCertSANs
		*out = make(ExtraAPIServerCertSANs, len(*in))
		copy(*out, *in)
	}
	in.ImageRegistries.DeepCopyInto(&out.ImageRegistries)
	if in.Addons != nil {
		in, out := &in.Addons, &out.Addons
		*out = new(Addons)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenericClusterConfig.
func (in *GenericClusterConfig) DeepCopy() *GenericClusterConfig {
	if in == nil {
		return nil
	}
	out := new(GenericClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPProxy) DeepCopyInto(out *HTTPProxy) {
	*out = *in
	if in.AdditionalNo != nil {
		in, out := &in.AdditionalNo, &out.AdditionalNo
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPProxy.
func (in *HTTPProxy) DeepCopy() *HTTPProxy {
	if in == nil {
		return nil
	}
	out := new(HTTPProxy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Image) DeepCopyInto(out *Image) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Image.
func (in *Image) DeepCopy() *Image {
	if in == nil {
		return nil
	}
	out := new(Image)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageRegistries) DeepCopyInto(out *ImageRegistries) {
	*out = *in
	if in.ImageRegistryCredentials != nil {
		in, out := &in.ImageRegistryCredentials, &out.ImageRegistryCredentials
		*out = make(ImageRegistryCredentials, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageRegistries.
func (in *ImageRegistries) DeepCopy() *ImageRegistries {
	if in == nil {
		return nil
	}
	out := new(ImageRegistries)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in ImageRegistryCredentials) DeepCopyInto(out *ImageRegistryCredentials) {
	{
		in := &in
		*out = make(ImageRegistryCredentials, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageRegistryCredentials.
func (in ImageRegistryCredentials) DeepCopy() ImageRegistryCredentials {
	if in == nil {
		return nil
	}
	out := new(ImageRegistryCredentials)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageRegistryCredentialsResource) DeepCopyInto(out *ImageRegistryCredentialsResource) {
	*out = *in
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(v1.ObjectReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageRegistryCredentialsResource.
func (in *ImageRegistryCredentialsResource) DeepCopy() *ImageRegistryCredentialsResource {
	if in == nil {
		return nil
	}
	out := new(ImageRegistryCredentialsResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NFD) DeepCopyInto(out *NFD) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NFD.
func (in *NFD) DeepCopy() *NFD {
	if in == nil {
		return nil
	}
	out := new(NFD)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectMeta) DeepCopyInto(out *ObjectMeta) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectMeta.
func (in *ObjectMeta) DeepCopy() *ObjectMeta {
	if in == nil {
		return nil
	}
	out := new(ObjectMeta)
	in.DeepCopyInto(out)
	return out
}
