// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	_ "embed"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
)

type StorageProvisioner string

const (
	CNIProviderCalico                     = "Calico"
	CNIProviderCilium                     = "Cilium"
	AWSEBSProvisioner  StorageProvisioner = "ebs.csi.aws.com"
	NutanixProvisioner StorageProvisioner = "csi.nutanix.com"

	CSIProviderAWSEBS  = "aws-ebs"
	CSIProviderNutanix = "nutanix"

	CCMProviderAWS     = "aws"
	CCMProviderNutanix = "nutanix"
)

var (
	DefaultDockerCertSANs = []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
		"host.docker.internal",
	}

	DefaultNutanixCertSANs = []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
	}

	//go:embed crds/caren.nutanix.com_dockerclusterconfigs.yaml
	dockerClusterConfigCRDDefinition []byte
	//go:embed crds/caren.nutanix.com_awsclusterconfigs.yaml
	awsClusterConfigCRDDefinition []byte
	//go:embed crds/caren.nutanix.com_nutanixclusterconfigs.yaml
	nutanixClusterConfigCRDDefinition []byte

	dockerClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		dockerClusterConfigCRDDefinition,
	)
	awsClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		awsClusterConfigCRDDefinition,
	)
	nutanixClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		nutanixClusterConfigCRDDefinition,
	)
)

// +kubebuilder:object:root=true

// AWSClusterConfig is the Schema for the awsclusterconfigs API.
type AWSClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec AWSClusterConfigSpec `json:"spec,omitempty"`
}

func (s AWSClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return awsClusterConfigVariableSchema
}

// AWSClusterConfigSpec defines the desired state of ClusterConfig.
type AWSClusterConfigSpec struct {
	// AWS cluster configuration.
	// +optional
	AWS *AWSSpec `json:"aws,omitempty"`

	GenericClusterConfigSpec `json:",inline"`

	// +optional
	ControlPlane *AWSControlPlaneNodeConfigSpec `json:"controlPlane,omitempty"`

	// Extra Subject Alternative Names for the API Server signing cert.
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +optional
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

// +kubebuilder:object:root=true

// DockerClusterConfig is the Schema for the dockerclusterconfigs API.
type DockerClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec DockerClusterConfigSpec `json:"spec,omitempty"`
}

func (s DockerClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return dockerClusterConfigVariableSchema
}

// DockerClusterConfigSpec defines the desired state of DockerClusterConfig.
type DockerClusterConfigSpec struct {
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`

	GenericClusterConfigSpec `json:",inline"`

	// +optional
	ControlPlane *DockerNodeConfigSpec `json:"controlPlane,omitempty"`

	// Extra Subject Alternative Names for the API Server signing cert.
	// For the Nutanix provider, the following default SANs will always be added:
	// - localhost
	// - 127.0.0.1
	// - 0.0.0.0
	// - host.docker.internal
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +optional
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

// +kubebuilder:object:root=true

// NutanixClusterConfig is the Schema for the nutanixclusterconfigs API.
type NutanixClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec NutanixClusterConfigSpec `json:"spec,omitempty"`
}

func (s NutanixClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return nutanixClusterConfigVariableSchema
}

// NutanixClusterConfigSpec defines the desired state of NutanixClusterConfig.
type NutanixClusterConfigSpec struct {
	// +optional
	Nutanix *NutanixSpec `json:"nutanix,omitempty"`

	GenericClusterConfigSpec `json:",inline"`

	// +optional
	ControlPlane *NutanixNodeConfigSpec `json:"controlPlane,omitempty"`

	// Subject Alternative Names for the API Server signing cert.
	// For the Nutanix provider, the following default SANs will always be added:
	// - localhost
	// - 127.0.0.1
	// - 0.0.0.0
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +optional
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

// GenericClusterConfigSpec defines the desired state of GenericClusterConfig.
type GenericClusterConfigSpec struct {
	// Sets the Kubernetes image repository used for the KubeadmControlPlane.
	// +kubebuilder:validation:Pattern=`^((?:[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*|\[(?:[a-fA-F0-9:]+)\])(:[0-9]+)?/)?[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*)*$`
	// +optional
	KubernetesImageRepository *string `json:"kubernetesImageRepository,omitempty"`

	// +optional
	Etcd *Etcd `json:"etcd,omitempty"`

	// +optional
	Proxy *HTTPProxy `json:"proxy,omitempty"`

	// +optional
	ImageRegistries []ImageRegistry `json:"imageRegistries,omitempty"`

	// +optional
	GlobalImageRegistryMirror *GlobalImageRegistryMirror `json:"globalImageRegistryMirror,omitempty"`

	// +optional
	Addons *Addons `json:"addons,omitempty"`

	// +optional
	Users []User `json:"users,omitempty"`
}

type Image struct {
	// Repository is used to override the image repository to pull from.
	// +kubebuilder:validation:Pattern=`^((?:[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*|\[(?:[a-fA-F0-9:]+)\])(:[0-9]+)?/)?[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*)*$`
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag is used to override the default image tag.
	// +kubebuilder:validation:Pattern=`^[\w][\w.-]{0,127}$`
	// +optional
	Tag string `json:"tag,omitempty"`
}

type Etcd struct {
	// Image required for overriding etcd image details.
	// +optional
	Image *Image `json:"image,omitempty"`
}

type RegistryCredentials struct {
	// A reference to the Secret containing the registry credentials and optional CA certificate
	// using the keys `username`, `password` and `ca.crt`.
	// This credentials Secret is not required for some registries, e.g. ECR.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// GlobalImageRegistryMirror sets default mirror configuration for all the image registries.
type GlobalImageRegistryMirror struct {
	// Registry mirror URL.
	// +kubebuilder:validation:Format=`uri`
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Credentials and CA certificate for the image registry mirror
	// +optional
	Credentials *RegistryCredentials `json:"credentials,omitempty"`
}

type ImageRegistry struct {
	// Registry URL.
	// +kubebuilder:validation:Format=`uri`
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Credentials and CA certificate for the image registry
	// +optional
	Credentials *RegistryCredentials `json:"credentials,omitempty"`
}

// User defines the input for a generated user in cloud-init.
type User struct {
	// Name specifies the user name.
	Name string `json:"name"`

	// HashedPassword is a hashed password for the user, formatted as described
	// by the crypt(5) man page. See your distribution's documentation for
	// instructions to create a hashed password.
	// An empty string is not marshalled, because it is not a valid value.
	// +optional
	HashedPassword string `json:"hashedPassword,omitempty"`

	// SSHAuthorizedKeys is a list of public SSH keys to write to the
	// machine. Use the corresponding private SSH keys to authenticate. See SSH
	// documentation for instructions to create a key pair.
	// +optional
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`

	// Sudo is a sudo user specification, formatted as described in the sudo
	// documentation.
	// An empty string is not marshalled, because it is not a valid value.
	// +optional
	Sudo string `json:"sudo,omitempty"`
}

func init() {
	SchemeBuilder.Register(
		&AWSClusterConfig{},
		&DockerClusterConfig{},
		&NutanixClusterConfig{},
	)
}
