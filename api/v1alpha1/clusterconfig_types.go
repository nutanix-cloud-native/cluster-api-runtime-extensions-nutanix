// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/nutanix-cloud-native/cluster-api-runtime-extensions-nutanix/api/openapi/patterns"
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
)

// +kubebuilder:object:root=true

// ClusterConfig is the Schema for the clusterconfigs API.
type ClusterConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec ClusterConfigSpec `json:"spec,omitempty"`
}

// ClusterConfigSpec defines the desired state of ClusterConfig.
type ClusterConfigSpec struct {
	// +optional
	AWS *AWSSpec `json:"aws,omitempty"`
	// +optional
	Docker *DockerSpec `json:"docker,omitempty"`
	// +optional
	Nutanix *NutanixSpec `json:"nutanix,omitempty"`

	GenericClusterConfig `json:",inline"`

	// +optional
	ControlPlane *NodeConfigSpec `json:"controlPlane,omitempty"`
}

func (s ClusterConfigSpec) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	clusterConfigProps := GenericClusterConfig{}.VariableSchema()
	switch {
	case s.AWS != nil:
		maps.Copy(
			clusterConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				AWSVariableName: s.AWS.VariableSchema().OpenAPIV3Schema,
				"controlPlane":  s.ControlPlane.VariableSchema().OpenAPIV3Schema,
			},
		)
	case s.Docker != nil:
		maps.Copy(
			clusterConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				"docker": DockerSpec{}.VariableSchema().OpenAPIV3Schema,
				"controlPlane": NodeConfigSpec{
					Docker: &DockerNodeSpec{},
				}.VariableSchema().OpenAPIV3Schema,
			},
		)
	case s.Nutanix != nil:
		maps.Copy(
			clusterConfigProps.OpenAPIV3Schema.Properties,
			map[string]clusterv1.JSONSchemaProps{
				NutanixVariableName: NutanixSpec{}.VariableSchema().OpenAPIV3Schema,
				"controlPlane": NodeConfigSpec{
					Nutanix: &NutanixNodeSpec{},
				}.VariableSchema().OpenAPIV3Schema,
			},
		)
	}

	return clusterConfigProps
}

func NewAWSClusterConfigSpec() *ClusterConfigSpec {
	return &ClusterConfigSpec{
		AWS: &AWSSpec{},
		ControlPlane: &NodeConfigSpec{
			AWS: NewAWSControlPlaneNodeSpec(),
		},
	}
}

// GenericClusterConfig defines the generic cluster configdesired.
type GenericClusterConfig struct {
	// +optional
	KubernetesImageRepository *KubernetesImageRepository `json:"kubernetesImageRepository,omitempty"`

	// +optional
	Etcd *Etcd `json:"etcd,omitempty"`

	// +optional
	Proxy *HTTPProxy `json:"proxy,omitempty"`

	// +optional
	ExtraAPIServerCertSANs ExtraAPIServerCertSANs `json:"extraAPIServerCertSANs,omitempty"`

	// +optional
	ImageRegistries ImageRegistries `json:"imageRegistries,omitempty"`

	// +optional
	GlobalImageRegistryMirror *GlobalImageRegistryMirror `json:"globalImageRegistryMirror,omitempty"`

	// +optional
	Addons *Addons `json:"addons,omitempty"`

	// +optional
	Users Users `json:"users,omitempty"`
}

func (s GenericClusterConfig) VariableSchema() clusterv1.VariableSchema { //nolint:gocritic,lll // Passed by value for no potential side-effect.
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Cluster configuration",
			Type:        "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"addons":                 Addons{}.VariableSchema().OpenAPIV3Schema,
				"etcd":                   Etcd{}.VariableSchema().OpenAPIV3Schema,
				"extraAPIServerCertSANs": ExtraAPIServerCertSANs{}.VariableSchema().OpenAPIV3Schema,
				"proxy":                  HTTPProxy{}.VariableSchema().OpenAPIV3Schema,
				"kubernetesImageRepository": KubernetesImageRepository(
					"",
				).VariableSchema().
					OpenAPIV3Schema,
				"imageRegistries":           ImageRegistries{}.VariableSchema().OpenAPIV3Schema,
				"globalImageRegistryMirror": GlobalImageRegistryMirror{}.VariableSchema().OpenAPIV3Schema,
				"users":                     Users{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

// KubernetesImageRepository required for overriding Kubernetes image repository.
type KubernetesImageRepository string

func (KubernetesImageRepository) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Sets the Kubernetes image repository used for the KubeadmControlPlane.",
			Type:        "string",
			Pattern:     patterns.Anchored(patterns.ImageRepository),
		},
	}
}

func (v KubernetesImageRepository) String() string {
	return string(v)
}

type Image struct {
	// Repository is used to override the image repository to pull from.
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag is used to override the default image tag.
	// +optional
	Tag string `json:"tag,omitempty"`
}

func (Image) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"repository": {
					Description: "Image repository to pull from.",
					Type:        "string",
					Pattern:     patterns.Anchored(patterns.ImageRepository),
				},
				"tag": {
					Description: "Image tag to use.",
					Type:        "string",
					Pattern:     patterns.Anchored(patterns.ImageTag),
				},
			},
		},
	}
}

type Etcd struct {
	// Image required for overriding etcd image details.
	// +optional
	Image *Image `json:"image,omitempty"`
}

func (Etcd) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"image": Image{}.VariableSchema().OpenAPIV3Schema,
			},
		},
	}
}

// HTTPProxy required for providing proxy configuration.
type HTTPProxy struct {
	// HTTP proxy.
	HTTP string `json:"http,omitempty"`

	// HTTPS proxy.
	HTTPS string `json:"https,omitempty"`

	// AdditionalNo Proxy list that will be added to the automatically calculated
	// values that will apply no_proxy configuration for cluster internal network.
	// Default values: localhost,127.0.0.1,<POD_NETWORK>,<SERVICE_NETWORK>,kubernetes
	//   ,kubernetes.default,.svc,.svc.<SERVICE_DOMAIN>
	AdditionalNo []string `json:"additionalNo"`
}

func (HTTPProxy) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"http": {
					Description: "HTTP proxy value.",
					Type:        "string",
				},
				"https": {
					Description: "HTTPS proxy value.",
					Type:        "string",
				},
				"additionalNo": {
					Description: "Additional No Proxy list that will be added to the automatically calculated " +
						"values required for cluster internal network. " +
						"Default value: localhost,127.0.0.1,<POD_NETWORK>,<SERVICE_NETWORK>,kubernetes," +
						"kubernetes.default,.svc,.svc.<SERVICE_DOMAIN>",
					Type: "array",
					Items: &clusterv1.JSONSchemaProps{
						Type: "string",
					},
				},
			},
		},
	}
}

// ExtraAPIServerCertSANs required for providing API server cert SANs.
type ExtraAPIServerCertSANs []string

func (ExtraAPIServerCertSANs) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: fmt.Sprintf(
				//nolint:lll // its a user facing message
				"Subject Alternative Names for the API Server signing cert. For Docker %s are injected automatically. For Nutanix %s are injected automatically.",
				strings.Join(DefaultDockerCertSANs, ","),
				strings.Join(DefaultNutanixCertSANs, ","),
			),
			Type:        "array",
			UniqueItems: true,
			Items: &clusterv1.JSONSchemaProps{
				Type:    "string",
				Pattern: patterns.Anchored(patterns.DNS1123Subdomain),
			},
		},
	}
}

type RegistryCredentials struct {
	// A reference to the Secret containing the registry credentials and optional CA certificate
	// using the keys `username`, `password` and `ca.crt`.
	// This credentials Secret is not required for some registries, e.g. ECR.
	// +optional
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

func (RegistryCredentials) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"secretRef": {
					Description: "A reference to the Secret containing the registry credentials and optional CA certificate. " +
						"The Secret should have keys 'username', 'password' and optional 'ca.crt'. " +
						"This credentials Secret is not required for some registries, e.g. ECR.",
					Type: "object",
					Properties: map[string]clusterv1.JSONSchemaProps{
						"name": {
							Description: "The name of the Secret containing the registry credentials. This Secret must exist in " +
								"the same namespace as the Cluster.",
							Type: "string",
						},
					},
					Required: []string{"name"},
				},
			},
		},
	}
}

// GlobalImageRegistryMirror sets default mirror configuration for all the image registries.
type GlobalImageRegistryMirror struct {
	// Registry URL.
	URL string `json:"url"`

	// Credentials and CA certificate for the image registry mirror
	// +optional
	Credentials *RegistryCredentials `json:"credentials,omitempty"`
}

func (GlobalImageRegistryMirror) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"url": {
					Description: "Registry mirror URL.",
					Type:        "string",
					Format:      "uri",
					Pattern:     "^https?://",
				},
				"credentials": RegistryCredentials{}.VariableSchema().OpenAPIV3Schema,
			},
			Required: []string{"url"},
		},
	}
}

type ImageRegistry struct {
	// Registry URL.
	URL string `json:"url"`

	// Credentials and CA certificate for the image registry
	// +optional
	Credentials *RegistryCredentials `json:"credentials,omitempty"`
}

func (ImageRegistry) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]clusterv1.JSONSchemaProps{
				"url": {
					Description: "Registry URL.",
					Type:        "string",
					Format:      "uri",
					Pattern:     "^https?://",
				},
				"credentials": RegistryCredentials{}.VariableSchema().OpenAPIV3Schema,
			},
			Required: []string{"url"},
		},
	}
}

type ImageRegistries []ImageRegistry

func (ImageRegistries) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Configuration for image registries.",
			Type:        "array",
			Items:       ptr.To(ImageRegistry{}.VariableSchema().OpenAPIV3Schema),
		},
	}
}

type Users []User

func (Users) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Description: "Users to add to the machine",
			Type:        "array",
			Items:       ptr.To(User{}.VariableSchema().OpenAPIV3Schema),
		},
	}
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

func (User) VariableSchema() clusterv1.VariableSchema {
	return clusterv1.VariableSchema{
		OpenAPIV3Schema: clusterv1.JSONSchemaProps{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]clusterv1.JSONSchemaProps{
				"name": {
					Description: "The username",
					Type:        "string",
				},
				"hashedPassword": {
					Description: "The hashed password for the user. Must be in the format of some hash function supported by the OS.",
					Type:        "string",
					// The crypt (5) man page lists regexes for supported hash
					// functions. We could validate input against a set of
					// regexes, but because the set may be different from the
					// set supported by the chosen OS, we might return a false
					// negative or positive. For this reason, we do not validate
					// the input.
				},
				"sshAuthorizedKeys": {
					Description: "A list of SSH authorized keys for this user",
					Type:        "array",
					Items: &clusterv1.JSONSchemaProps{
						// No description, because the one for the parent array is enough.
						Type: "string",
					},
				},
				"sudo": {
					Description: "The sudo rule that applies to this user",
					Type:        "string",
					// A sudo rule is defined using an EBNF grammar, and must be
					// parsed to be validated. We have decided to not integrate
					// a sudo rule parser, so we do not validate the input.
				},
			},
		},
	}
}

func init() {
	SchemeBuilder.Register(&ClusterConfig{})
}
