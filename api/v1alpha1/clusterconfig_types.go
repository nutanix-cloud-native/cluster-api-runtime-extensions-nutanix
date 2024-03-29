// Copyright 2023 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/d2iq-labs/cluster-api-runtime-extensions-nutanix/api/openapi/patterns"
)

const (
	CNIProviderCalico = "Calico"
	CNIProviderCilium = "Cilium"

	CSIProviderAWSEBS = "aws-ebs"

	CCMProviderAWS = "aws"
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
				AWSVariableName: AWSSpec{}.VariableSchema().OpenAPIV3Schema,
				"controlPlane": NodeConfigSpec{
					AWS: &AWSNodeSpec{},
				}.VariableSchema().OpenAPIV3Schema,
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
	}

	return clusterConfigProps
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
			Description: "Extra Subject Alternative Names for the API Server signing cert",
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

func init() {
	SchemeBuilder.Register(&ClusterConfig{})
}
