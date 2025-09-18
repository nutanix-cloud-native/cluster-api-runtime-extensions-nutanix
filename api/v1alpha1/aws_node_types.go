// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	capav1 "github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/external/sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
)

type AWSControlPlaneNodeSpec struct {
	// The IAM instance profile to use for the cluster Machines.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=control-plane.cluster-api-provider-aws.sigs.k8s.io
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=m5.xlarge
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	InstanceType string `json:"instanceType,omitempty"`

	AWSGenericNodeSpec `json:",inline"`
}

type AWSWorkerNodeSpec struct {
	// The IAM instance profile to use for the cluster Machines.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=nodes.cluster-api-provider-aws.sigs.k8s.io
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// The AWS instance type to use for the cluster Machines.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=m5.2xlarge
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	InstanceType string `json:"instanceType,omitempty"`

	AWSGenericNodeSpec `json:",inline"`
}

type AWSGenericNodeSpec struct {
	// AMI or AMI Lookup arguments for machine image of a AWS machine.
	// If both AMI ID and AMI lookup arguments are provided then AMI ID takes precedence
	// +kubebuilder:validation:Optional
	AMISpec *AMISpec `json:"ami,omitempty"`

	// +kubebuilder:validation:Optional
	AdditionalSecurityGroups AdditionalSecurityGroup `json:"additionalSecurityGroups,omitempty"`

	// PlacementGroup specifies the placement group in which to launch the instance.
	// +kubebuilder:validation:Optional
	PlacementGroup *PlacementGroup `json:"placementGroup,omitempty"`

	// Configuration options for the root and additional storage volume.
	// +kubebuilder:validation:Optional
	Volumes *AWSVolumes `json:"volumes,omitempty"`
}

// +kubebuilder:validation:MaxItems=32
type AdditionalSecurityGroup []SecurityGroup

type PlacementGroup struct {
	// Name is the name of the placement group.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Name string `json:"name"`
}

type SecurityGroup struct {
	// ID is the id of the security group
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=`^sg-[0-9a-f]{8}(?:[0-9a-f]{9})?$`
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id,omitempty"`
}

type AMISpec struct {
	// AMI ID is the reference to the AMI from which to create the machine instance.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=`^ami-[0-9a-f]{8}(?:[0-9a-f]{9})?$`
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id,omitempty"`

	// Lookup is the lookup arguments for the AMI.
	// +kubebuilder:validation:Optional
	Lookup *AMILookup `json:"lookup,omitempty"`
}

type AMILookup struct {
	// AMI naming format. Supports substitutions for {{.BaseOS}} and {{.K8sVersion}} with the
	// base OS and kubernetes version.
	// +kubebuilder:validation:Optional
	// +kubebuilder:example=`capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Format string `json:"format,omitempty"`

	// The AWS Organization ID to use for image lookup.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Format=`^o-[0-9a-z]{10,32}$`
	// +kubebuilder:validation:MinLength=12
	// +kubebuilder:validation:MaxLength=34
	Org string `json:"org,omitempty"`

	// The name of the base os for image lookup
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=32
	BaseOS string `json:"baseOS,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.root) || size(self.nonroot) > 0",message="either root or nonroot must be specified"
type AWSVolumes struct {
	// Configuration options for the root storage volume.
	// +kubebuilder:validation:Optional
	Root *AWSVolume `json:"root,omitempty"`

	// Configuration options for non-root storage volumes.
	// +kubebuilder:validation:Optional
	NonRoot []AWSVolume `json:"nonroot,omitempty"`
}

type AWSVolume struct {
	// Device name
	// +kubebuilder:validation:Optional
	DeviceName string `json:"deviceName,omitempty"`

	// Size specifies size (in Gi) of the storage device.
	// Must be greater than the image snapshot size or 8 (whichever is greater).
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=8
	Size int64 `json:"size,omitempty"`

	// Type is the type of the volume (e.g. gp2, io1, etc...).
	// +kubebuilder:validation:Optional
	Type capav1.VolumeType `json:"type,omitempty"`

	// IOPS is the number of IOPS requested for the disk. Not applicable to all types.
	// +kubebuilder:validation:Optional
	IOPS int64 `json:"iops,omitempty"`

	// Throughput to provision in MiB/s supported for the volume type. Not applicable to all types.
	// +kubebuilder:validation:Optional
	Throughput int64 `json:"throughput,omitempty"`

	// Encrypted is whether the volume should be encrypted or not.
	// +kubebuilder:validation:Optional
	Encrypted bool `json:"encrypted,omitempty"`

	// EncryptionKey is the KMS key to use to encrypt the volume. Can be either a KMS key ID or ARN.
	// If Encrypted is set and this is omitted, the default AWS key will be used.
	// The key must already exist and be accessible by the controller.
	// +kubebuilder:validation:Optional
	EncryptionKey string `json:"encryptionKey,omitempty"`
}
