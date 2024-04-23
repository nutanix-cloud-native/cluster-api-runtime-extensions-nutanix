// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type AWSControlPlaneNodeSpec struct {
	// The IAM instance profile to use for the cluster Machines.
	// +kubebuilder:default=control-plane.cluster-api-provider-aws.sigs.k8s.io
	// +optional
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// +kubebuilder:default=m5.xlarge
	// +optional
	InstanceType string `json:"instanceType,omitempty"`

	AWSGenericNodeSpec `json:",inline"`
}

type AWSWorkerNodeSpec struct {
	// The IAM instance profile to use for the cluster Machines.
	// +kubebuilder:default=nodes.cluster-api-provider-aws.sigs.k8s.io
	// +optional
	IAMInstanceProfile string `json:"iamInstanceProfile,omitempty"`

	// The AWS instance type to use for the cluster Machines.
	// +kubebuilder:default=m5.2xlarge
	// +optional
	InstanceType string `json:"instanceType,omitempty"`

	AWSGenericNodeSpec `json:",inline"`
}

type AWSGenericNodeSpec struct {
	// AMI or AMI Lookup arguments for machine image of a AWS machine.
	// If both AMI ID and AMI lookup arguments are provided then AMI ID takes precedence
	//+optional
	AMISpec *AMISpec `json:"ami,omitempty"`

	//+optional
	AdditionalSecurityGroups AdditionalSecurityGroup `json:"additionalSecurityGroups,omitempty"`
}

type AdditionalSecurityGroup []SecurityGroup

type SecurityGroup struct {
	// ID is the id of the security group
	// +optional
	ID string `json:"id,omitempty"`
}

type AMISpec struct {
	// AMI ID is the reference to the AMI from which to create the machine instance.
	// +optional
	ID string `json:"id,omitempty"`

	// Lookup is the lookup arguments for the AMI.
	// +optional
	Lookup *AMILookup `json:"lookup,omitempty"`
}

type AMILookup struct {
	// AMI naming format. Supports substitutions for {{.BaseOS}} and {{.K8sVersion}} with the
	// base OS and kubernetes version.
	// +kubebuilder:example=`capa-ami-{{.BaseOS}}-?{{.K8sVersion}}-*`
	// +optional
	Format string `json:"format,omitempty"`

	// The AWS Organization ID to use for image lookup.
	// +optional
	Org string `json:"org,omitempty"`

	// The name of the base os for image lookup
	// +optional
	BaseOS string `json:"baseOS,omitempty"`
}
