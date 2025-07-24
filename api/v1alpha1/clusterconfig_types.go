// Copyright 2023 Nutanix. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	_ "embed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/common/pkg/capi/clustertopology/variables"
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
	//go:embed crds/caren.nutanix.com_genericclusterconfigs.yaml
	genericClusterConfigCRDDefinition []byte

	dockerClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		dockerClusterConfigCRDDefinition,
	)
	awsClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		awsClusterConfigCRDDefinition,
	)
	nutanixClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		nutanixClusterConfigCRDDefinition,
	)
	genericClusterConfigVariableSchema = variables.MustSchemaFromCRDYAML(
		genericClusterConfigCRDDefinition,
	)
)

// +kubebuilder:object:root=true

// AWSClusterConfig is the Schema for the awsclusterconfigs API.
type AWSClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec *AWSClusterConfigSpec `json:"spec,omitempty"`
}

func (s AWSClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return awsClusterConfigVariableSchema
}

// AWSClusterConfigSpec defines the desired state of ClusterConfig.
type AWSClusterConfigSpec struct {
	// AWS cluster configuration.
	// +kubebuilder:validation:Optional
	AWS *AWSSpec `json:"aws,omitempty"`

	GenericClusterConfigSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Addons *AWSAddons `json:"addons,omitempty"`

	// +kubebuilder:validation:Optional
	ControlPlane *AWSControlPlaneSpec `json:"controlPlane,omitempty"`

	// Extra Subject Alternative Names for the API Server signing cert.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=253
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

// +kubebuilder:object:root=true

// DockerClusterConfig is the Schema for the dockerclusterconfigs API.
type DockerClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec *DockerClusterConfigSpec `json:"spec,omitempty"`
}

func (s DockerClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return dockerClusterConfigVariableSchema
}

// DockerClusterConfigSpec defines the desired state of DockerClusterConfig.
type DockerClusterConfigSpec struct {
	// +kubebuilder:validation:Optional
	Docker *DockerSpec `json:"docker,omitempty"`

	GenericClusterConfigSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Addons *DockerAddons `json:"addons,omitempty"`

	// +kubebuilder:validation:Optional
	ControlPlane *DockerControlPlaneSpec `json:"controlPlane,omitempty"`

	// Extra Subject Alternative Names for the API Server signing cert.
	// For the Docker provider, the following default SANs will always be added:
	// - localhost
	// - 127.0.0.1
	// - 0.0.0.0
	// - host.docker.internal
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=253
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

// +kubebuilder:object:root=true

// NutanixClusterConfig is the Schema for the nutanixclusterconfigs API.
type NutanixClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec *NutanixClusterConfigSpec `json:"spec,omitempty"`
}

func (s NutanixClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return nutanixClusterConfigVariableSchema
}

// NutanixClusterConfigSpec defines the desired state of NutanixClusterConfig.
type NutanixClusterConfigSpec struct {
	// +kubebuilder:validation:Optional
	Nutanix *NutanixSpec `json:"nutanix,omitempty"`

	GenericClusterConfigSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Addons *NutanixAddons `json:"addons,omitempty"`

	// +kubebuilder:validation:Optional
	ControlPlane *NutanixControlPlaneSpec `json:"controlPlane,omitempty"`

	// Subject Alternative Names for the API Server signing cert.
	// For the Nutanix provider, the following default SANs will always be added:
	// - localhost
	// - 127.0.0.1
	// - 0.0.0.0
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=253
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

// +kubebuilder:object:root=true

// GenericClusterConfig is the Schema for the genericclusterconfigs API.
type GenericClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Optional
	Spec *GenericClusterConfigSpec `json:"spec,omitempty"`

	// Extra Subject Alternative Names for the API Server signing cert.
	// +kubebuilder:validation:UniqueItems=true
	// +kubebuilder:validation:items:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=100
	// +kubebuilder:validation:items:MaxLength=253
	ExtraAPIServerCertSANs []string `json:"extraAPIServerCertSANs,omitempty"`
}

func (s GenericClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return genericClusterConfigVariableSchema
}

// GenericClusterConfigSpec defines the desired state of GenericClusterConfig.
type GenericClusterConfigSpec struct {
	// Sets the Kubernetes image repository used for the KubeadmControlPlane.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^((?:[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*|\[(?:[a-fA-F0-9:]+)\])(:[0-9]+)?/)?[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*)*$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	KubernetesImageRepository string `json:"kubernetesImageRepository,omitempty"`

	// +kubebuilder:validation:Optional
	Etcd *Etcd `json:"etcd,omitempty"`

	// +kubebuilder:validation:Optional
	Proxy *HTTPProxy `json:"proxy,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	ImageRegistries []ImageRegistry `json:"imageRegistries,omitempty"`

	// +kubebuilder:validation:Optional
	GlobalImageRegistryMirror *GlobalImageRegistryMirror `json:"globalImageRegistryMirror,omitempty"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	Users []User `json:"users,omitempty"`

	// +kubebuilder:validation:Optional
	EncryptionAtRest *EncryptionAtRest `json:"encryptionAtRest,omitempty"`

	// +kubebuilder:validation:Optional
	DNS *DNS `json:"dns,omitempty"`

	// KubeProxy defines the configuration for kube-proxy.
	// +kubebuilder:validation:Optional
	KubeProxy *KubeProxy `json:"kubeProxy,omitempty"`

	// NTP defines the NTP configuration for the cluster.
	// +kubebuilder:validation:Optional
	NTP *NTP `json:"ntp,omitempty"`
}

type Image struct {
	// Repository is used to override the image repository to pull from.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^((?:[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*|\[(?:[a-fA-F0-9:]+)\])(:[0-9]+)?/)?[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*(/[a-z0-9]+((?:[._]|__|[-]+)[a-z0-9]+)*)*$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=2048
	Repository string `json:"repository,omitempty"`

	// Tag is used to override the default image tag.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[\w][\w.-]{0,127}$`
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=128
	Tag string `json:"tag,omitempty"`
}

type Etcd struct {
	// Image required for overriding etcd image details.
	// +kubebuilder:validation:Optional
	Image *Image `json:"image,omitempty"`
}

type RegistryCredentials struct {
	// A reference to the Secret containing the registry credentials and optional CA certificate
	// using the keys `username`, `password` and `ca.crt`.
	// This credentials Secret is not required for some registries, e.g. ECR.
	// +kubebuilder:validation:Optional
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
}

// GlobalImageRegistryMirror sets default mirror configuration for all the image registries.
type GlobalImageRegistryMirror struct {
	// Registry mirror URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=`uri`
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Credentials and CA certificate for the image registry mirror
	// +kubebuilder:validation:Optional
	Credentials *RegistryCredentials `json:"credentials,omitempty"`
}

type ImageRegistry struct {
	// Registry URL.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=`uri`
	// +kubebuilder:validation:Pattern=`^https?://`
	URL string `json:"url"`

	// Credentials and CA certificate for the image registry
	// +kubebuilder:validation:Optional
	Credentials *RegistryCredentials `json:"credentials,omitempty"`
}

// User defines the input for a generated user in cloud-init.
type User struct {
	// Name specifies the user name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=256
	Name string `json:"name"`

	// HashedPassword is a hashed password for the user, formatted as described
	// by the crypt(5) man page. See your distribution's documentation for
	// instructions to create a hashed password.
	// An empty string is not marshalled, because it is not a valid value.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=106
	HashedPassword string `json:"hashedPassword,omitempty"`

	// SSHAuthorizedKeys is a list of public SSH keys to write to the
	// machine. Use the corresponding private SSH keys to authenticate. See SSH
	// documentation for instructions to create a key pair.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:items:MaxLength=256
	SSHAuthorizedKeys []string `json:"sshAuthorizedKeys,omitempty"`

	// Sudo is a sudo user specification, formatted as described in the sudo
	// documentation.
	// An empty string is not marshalled, because it is not a valid value.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Sudo string `json:"sudo,omitempty"`
}

// EncryptionAtRest defines the configuration to enable encryption at REST
// This configuration is used by API server to encrypt data before storing it in ETCD.
// Currently the encryption only enabled for secrets and configmaps.
type EncryptionAtRest struct {
	// Encryption providers
	// +kubebuilder:default={{aescbc:{}}}
	// +kubebuilder:validation:MaxItems=1
	// +kubebuilder:validation:Optional
	Providers []EncryptionProviders `json:"providers,omitempty"`
}

type EncryptionProviders struct {
	// +kubebuilder:validation:Optional
	AESCBC *AESConfiguration `json:"aescbc,omitempty"`
	// +kubebuilder:validation:Optional
	Secretbox *SecretboxConfiguration `json:"secretbox,omitempty"`
}

type AESConfiguration struct{}

type SecretboxConfiguration struct{}

// DNS defines the DNS configuration for the cluster.
type DNS struct {
	// CoreDNS defines the CoreDNS configuration for the cluster.
	// +kubebuilder:validation:Optional
	CoreDNS *CoreDNS `json:"coreDNS,omitempty"`
}

type CoreDNS struct {
	// Image required for overriding Kubernetes DNS image details.
	// If the image version is not specified,
	// the default version based on the cluster's Kubernetes version will be used.
	// +kubebuilder:validation:Optional
	Image *Image `json:"image,omitempty"`
}

type KubeProxyMode string

const (
	// KubeProxyModeIPTables indicates that kube-proxy should be installed in iptables
	// mode.
	KubeProxyModeIPTables KubeProxyMode = "iptables"
	// KubeProxyModeNFTables indicates that kube-proxy should be installed in nftables
	// mode.
	KubeProxyModeNFTables KubeProxyMode = "nftables"
)

type KubeProxy struct {
	// Mode specifies the mode for kube-proxy:
	// - iptables means that kube-proxy is installed in iptables mode.
	// - nftables means that kube-proxy is installed in nftables mode.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=iptables;nftables
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value cannot be changed after cluster creation"
	Mode KubeProxyMode `json:"mode,omitempty"`
}

// NTP defines the NTP configuration for the cluster.
type NTP struct {
	// Servers is a list of NTP servers to use for time synchronization.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:items:MaxLength=253
	Servers []string `json:"servers"`
}

//nolint:gochecknoinits // Idiomatic to use init functions to register APIs with scheme.
func init() {
	objectTypes = append(objectTypes,
		&AWSClusterConfig{},
		&DockerClusterConfig{},
		&NutanixClusterConfig{},
	)
}
