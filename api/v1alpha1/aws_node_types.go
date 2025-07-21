// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type AWSControlPlaneNodeSpec struct {
	// The IAM instance profile to use for the cluster Machines.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=control-plane.cluster-api-provider-aws.sigs.k8s.io
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=m5.xlarge
	InstanceType string `json:"instanceType,omitempty"`

	AWSGenericNodeSpec `json:",inline"`
}

type AWSWorkerNodeSpec struct {
	// The IAM instance profile to use for the cluster Machines.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=nodes.cluster-api-provider-aws.sigs.k8s.io
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// The AWS instance type to use for the cluster Machines.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=m5.2xlarge
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
	PlacementGroup *PlacementGroup `json:"placementGroupName,omitempty"`
}

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
	ID string `json:"id,omitempty"`
}

type AMISpec struct {
	// AMI ID is the reference to the AMI from which to create the machine instance.
	// +kubebuilder:validation:Optional
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
	Format string `json:"format,omitempty"`

	// The AWS Organization ID to use for image lookup.
	// +kubebuilder:validation:Optional
	Org string `json:"org,omitempty"`

	// The name of the base os for image lookup
	// +kubebuilder:validation:Optional
	BaseOS string `json:"baseOS,omitempty"`
}
